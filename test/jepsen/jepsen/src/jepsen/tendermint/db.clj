(ns jepsen.tendermint.db
  "Supports tendermint operations like installation, creating validators,
  starting and stopping nodes, etc."
  (:require [clojure.tools.logging :refer :all]
            [clojure.java.io :as io]
            [clojure.string :as str]
            [clojure.pprint :refer [pprint]]
            [slingshot.slingshot :refer [try+]]
            [jepsen [core :as jepsen]
             [control :as c]
             [db :as db]
             [util :as util :refer [timeout with-retry map-vals]]]
            [jepsen.control.util :as cu]
            [jepsen.os.debian :as debian]
            [jepsen.nemesis.time :as nt]
            [cheshire.core :as json]
            [jepsen.tendermint [client :as tc]
             [util :refer [base-dir]]
             [validator :as tv]]))

(defn install-component!
  "Download and install a tendermint component"
  [app opts]
  (let [opt-name (keyword (str app "-url"))
        path (get opts opt-name)]
    (cu/install-archive! path (str base-dir "/" app))))

(defn write-validator!
  "Writes out the given validator structure to priv_validator.json."
  [validator]
  (c/su
   (c/cd base-dir
         (c/exec :echo (json/generate-string validator)
                 :> "priv_validator_key.json")
         (info "Wrote priv_validator_key.json"))))

(defn write-genesis!
  "Writes a genesis structure to a JSON file on disk."
  [genesis]
  (c/su
   (c/cd base-dir
         (c/exec :echo (json/generate-string genesis)
                 :> "genesis.json")
         (info "Wrote genesis.json"))))

(defn write-config!
  "Writes out a config.toml file to the current node."
  []
  (c/su
   (c/cd base-dir
         (c/exec :echo (slurp (io/resource "config.toml"))
                 :> "config.toml"))))

(defn seeds
  "Constructs a --seeds command line for a test, so a tendermint node knows
  what other nodes to talk to."
  [test node]
  (->> (:nodes test)
       (remove #{node})
       (map (fn [node] (str node ":26656")))
       (str/join ",")))

(def socket-file (str base-dir "/merkleeyes.sock"))
(def socket
  "The socket address we use to communicate with merkleeyes"
  (str "unix://" socket-file))

(def merkleeyes-logfile (str base-dir "/merkleeyes.log"))
(def tendermint-logfile (str base-dir "/tendermint.log"))
(def merkleeyes-pidfile (str base-dir "/merkleeyes.pid"))
(def tendermint-pidfile (str base-dir "/tendermint.pid"))

(defn start-tendermint!
  "Starts tendermint as a daemon."
  [test node]
  (c/su
   (c/cd base-dir
         (cu/start-daemon!
          {:logfile tendermint-logfile
           :pidfile tendermint-pidfile
           :chdir   base-dir}
          "./tendermint"
          :--home base-dir
          :node
          :--proxy_app socket
          :--p2p.seeds (seeds test node))))
  :started)

(defn start-merkleeyes!
  "Starts merkleeyes as a daemon."
  [test node]
  (c/su
   (c/cd base-dir
         (cu/start-daemon!
          {:logfile merkleeyes-logfile
           :pidfile merkleeyes-pidfile
           :chdir   base-dir}
          "./merkleeyes"
          :start
          :--dbName   "jepsen"
          :--address  socket)))
  :started)

(defn stop-tendermint! [test node]
  (c/su (cu/stop-daemon! tendermint-pidfile))
  :stopped)

(defn stop-merkleeyes! [test node]
  (c/su (cu/stop-daemon! merkleeyes-pidfile)
        (c/exec :rm :-rf socket-file))
  :stopped)

(defn start!
  [test node]
  (start-merkleeyes! test node)
  (start-tendermint! test node))

(defn stop!
  [test node]
  (stop-merkleeyes! test node)
  (stop-tendermint! test node))

(def node-files
  "Files required for a validator's state."
  (map (partial str base-dir "/")
       ["data"
        "jepsen"
        "priv_validator.json"
        "priv_validator.json.bak"]))

(defn reset-node!
  "Wipe data files and identity but preserve binaries."
  [test node]
  (c/su (c/exec :rm :-rf node-files)))

(defn db
  "A complete Tendermint system. Options:

      :merkleeyes-url   Package URLs for Tendermint components
      :tendermint-url
      :abci-url

      :validator-config   An atom which the DB will fill in with the initial
                          validator config."
  [opts]
  (reify db/DB
    (setup! [_ test node]
      (c/su
       (install-component! "tendermint"  opts)
       (install-component! "abci"        opts)
       (install-component! "merkleeyes"  opts)

       (write-config!)

        ; OK we're ready to compute the initial validator config.
       (jepsen/synchronize test)
       (when (= node (jepsen/primary test))
         (let [validator-config (tv/initial-config test)]
           (info :initial-config (with-out-str (pprint validator-config)))
           (assert (compare-and-set! (:validator-config opts)
                                     nil
                                     validator-config)
                   "Initial validator config already established!")))

        ; Now apply that config.
       (jepsen/synchronize test)
       (let [vc @(:validator-config opts)]
         (write-genesis! (tv/genesis vc))
         (write-validator! (get (:validators vc)
                                (get-in vc [:nodes node]))))

       (start-merkleeyes! test node)
       (start-tendermint! test node)

       (nt/install!)

       (Thread/sleep 1000)))

    (teardown! [_ test node]
      (stop-merkleeyes! test node)
      (stop-tendermint! test node)
      (c/su
       (c/exec :rm :-rf base-dir)))

    db/LogFiles
    (log-files [_ test node]
      [tendermint-logfile
       merkleeyes-logfile
       (str base-dir "/priv_validator.json")
       (str base-dir "/genesis.json")])))
