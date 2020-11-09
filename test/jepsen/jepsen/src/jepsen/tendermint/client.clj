(ns jepsen.tendermint.client
  "Client for merkleeyes."
  (:refer-clojure :exclude [read])
  (:require [jepsen.tendermint.gowire :as w]
            [clojure.data.fressian :as f]
            [clj-http.client :as http]
            [cheshire.core :as json]
            [clojure.tools.logging :refer [info warn]]
            [jepsen.util :refer [map-vals]]
            [slingshot.slingshot :refer [throw+]]
            [byte-streams :as bs])
  (:import (java.nio ByteBuffer)
           (java.util Random)
           (java.lang StringBuilder)))

; General-purpose ABCI operations

(defn byte-buf->hex
  "Convert a byte buffer to a hex string."
  [^ByteBuffer buf]
  (let [sb (StringBuilder.)]
    (loop []
      (if (.hasRemaining buf)
        (do (.append sb (format "%02x" (bit-and (.get buf) 0xff)))
            (recur))
        (str sb)))))

(defn hex->byte-buf
  "Convert a string of hex digits to a byte buffer"
  [s]
  (let [n (/ (count s) 2)
        a (byte-array n)]
    (loop [i 0]
      (when (< i n)
        (aset a i (unchecked-byte (Integer/parseInt
                                   (subs s (* i 2) (+ (* i 2) 2)) 16)))
        (recur (inc i))))
    (ByteBuffer/wrap a)))

(defn encode-query-param
  "Encodes a string or bytebuffer for use as a URL parameter. Converts strings
  to quoted strings, e.g. \"foo\" -> \"\\\"foo\\\"\", and byte buffers to 0x...
  strings, e.g. \"0xabcd\"."
  [x]
  (condp instance? x
    String     (str "\"" x "\"")
    ByteBuffer (str "0x" (byte-buf->hex x))))

(defn validate-tx-code
  "Checks a check_tx or deliver_tx structure for errors, and throws as
  necessary.  Returns the tx otherwise."
  [tx]
  (case (:code tx)
    0   tx
    4   (throw+ {:type :unauthorized :log (:log tx)})
    111 (throw+ {:type :base-unknown-address, :log (:log tx)})
    (throw+ (assoc tx :type :unknown-tx-error))))

(def port "HTTP interface port" 26657)

(def default-http-opts
  "clj-http options"
  {:socket-timeout  10000
   :conn-timeout    10000
   :accept          :json
   :as              :json
   :throw-entire-message? true
   :retry-handler   (fn [ex try-count http-context] false)})

(defn broadcast-tx!
  "Broadcast a given transaction to the given node. tx can be a string, in
  which case it is encoded as \"...\", or a ByteBuffer, in which case it is
  encoded as 0x.... Throws for errors in either check_tx or deliver_tx, and if
  no errors are present, returns a result map."
  [node tx]
  (let [tx (encode-query-param tx)
        http-res (http/get (str "http://" node ":" port "/broadcast_tx_commit")
                           (assoc default-http-opts
                                  :query-params {:tx tx}))
        result (-> http-res :body :result)]
    ; (info :result result)
    (validate-tx-code (:check_tx result))
    (validate-tx-code (:deliver_tx result))
    result))

(defn abci-query
  "Performs an ABCI query on the given node."
  [node path data]
  (http/get (str "http://" node ":" port "/abci_query")
            (assoc default-http-opts
                   :query-params  {:data  (encode-query-param data)
                                   :path  (encode-query-param path)
                                   :prove false})))

; Merkleeyes-specific paths

(defn nonce
  "A 12 byte random nonce byte buffer"
  []
  (let [buf (byte-array 12)]
    (.nextBytes (Random.) buf)
    (w/fixed-bytes buf)))

(def tx-types
  "A map of transaction type keywords to their magic bytes."
  (map-vals w/uint8
            {:set                   0x01
             :remove                0x02
             :get                   0x03
             :cas                   0x04
             :validator-set-change  0x05
             :validator-set-read    0x06
             :validator-set-cas     0x07}))

(defn tx-type
  "Returns the byte for a transaction type keyword"
  [type-kw]
  (or (get tx-types type-kw)
      (throw (IllegalArgumentException. (str "Unknown tx type " type-kw)))))

(defn tx
  "Construct a merkleeyes transaction byte buffer"
  [type & args]
  (w/write [(nonce) (tx-type type) args]))

(defn write!
  "Ask node to set k to v"
  [node k v]
  (broadcast-tx! node (tx :set (f/write k) (f/write v))))

(defn read
  "Perform a transactional read"
  [node k]
  (-> (broadcast-tx! node (tx :get (f/write k)))
      :deliver_tx
      :data
      hex->byte-buf
      f/read))

(defn cas!
  "Perform a compare-and-set from v to v' of k"
  [node k v v']
  (broadcast-tx! node (tx :cas (f/write k) (f/write v) (f/write v'))))

(defn validator-set
  "Reads the current validator set, transactionally."
  [node]
  (-> (broadcast-tx! node (tx :validator-set-read))
      :deliver_tx
      :data
      hex->byte-buf
      (bs/convert java.io.Reader)
      (json/parse-stream true)))

(defn validator-set-change!
  "Change the weight of a validator, given by private key (a hex string), and a
  voting power, an integer."
  [node validator-key weight]
  (-> (broadcast-tx! node (tx :validator-set-change
                              (hex->byte-buf validator-key)
                              (w/uint64 weight)))))

(defn validator-set-cas!
  "Change the weight of a validator, iff the current version is as given."
  [node version validator-key power]
  (-> (broadcast-tx! node (tx :validator-set-cas
                              (w/uint64 version)
                              (hex->byte-buf validator-key)
                              (w/uint64 power)))))

(defn local-read
  "Read by querying a particular node"
  [node k]
  (let [k (f/write k)
        res (-> (abci-query node "/store" k)
                :body
                :result
                :response
                :value)]
    (if (= res "")
      nil
      (f/read (hex->byte-buf res)))))

(defn with-any-node
  "Takes a test, a function taking a node as its first argument, and remaining
  args to that function. Makes the request to various nodes until one
  connects."
  [test f & args]
  (reduce (fn [_ node]
            (try
              (reduced (apply f node args))
              (catch java.net.ConnectException e
                (condp re-find (.getMessage e)
                  #"Connection refused" nil
                  (throw e)))))
          nil
          (shuffle (:nodes test))))
