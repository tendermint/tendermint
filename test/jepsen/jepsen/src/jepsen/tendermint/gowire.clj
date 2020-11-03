(ns jepsen.tendermint.gowire
  "A verrrrry minimal implementation of Tendermint's custom serialization
  format: https://github.com/tendermint/go-wire"
  (:import (java.nio ByteBuffer)))

(defprotocol Writable
  (byte-size [w])
  (write! [w ^ByteBuffer b]))

; Fixed size ints
(defrecord UInt8  [^byte x]
  Writable
  (byte-size [_] 1)
  (write! [_ b] (.put b x) b))

(defn uint8
  [b]
  (assert (not (neg? b)))
  (UInt8. (unchecked-byte b)))

(defrecord UInt64 [^long x]
  Writable
  (byte-size [_] 8)
  (write! [_ b] (.putLong b x) b))

(defn uint64 [^long l]
  (assert (not (neg? l)))
  (UInt64. l))

; Fixed-size byte buffers, which don't have a length header
(defrecord FixedBytes [^ByteBuffer x]
  Writable
  (byte-size [_] (.remaining x))
  (write! [_ buf]
          (.put buf x)
          buf))

(defn fixed-bytes
  [x]
  (if (instance? ByteBuffer x)
    (FixedBytes. x)
    (FixedBytes. (ByteBuffer/wrap x))))

; Generic types
(extend-protocol Writable
  ; Nil is nothing
  nil
  (byte-size  [n] 0)
  (write!     [n buf] buf)

  ; Integers work like Longs
  Integer
  (byte-size  [n]  (byte-size (long n)))
  (write!     [n buf] (write! (long n) buf))

  ; Longs are varints
  Long
  (byte-size [n]
    (cond (<  n 0x0)        (throw (IllegalArgumentException.
                                     (str "Number " n " can't be negative")))
          (<= n 0x0)        1
          (<= n 0xff)       2
          (<= n 0xffff)     3
          (<= n 0xffffff)   4
          (<= n 0xffffffff) 5
          true              (throw (IllegalArgumentException.
                                     (str "Number " n " is too large")))))

  (write! [n buf]
    (let [int-size (dec (byte-size n))]
      (.put buf (unchecked-byte int-size)) ; Write size byte
      (condp = int-size
        0  nil
        1 (.put       buf (unchecked-byte n))
        2 (.putShort  buf (unchecked-short n))
        3 (throw (IllegalArgumentException. "Todo: bit munging"))
        4 (.putInt    buf (unchecked-int n))))
    buf)

  ByteBuffer
  (byte-size [b]
    (let [r (.remaining b)]
      (+ (byte-size r) r)))

  (write! [inbuf outbuf]
    (write! (.remaining inbuf) outbuf)
    (.put outbuf inbuf)
    outbuf)

  ; To write sequential collections, just write each thing recursively
  clojure.lang.Sequential
  (byte-size [coll]
    (reduce + 0 (map byte-size coll)))

  (write! [coll buf]
    (doseq [x coll]
      (write! x buf))
    buf))

(defn buffer-for
  "Constructs a ByteBuffer big enough to write the given collection of objects
  to. Extra is an integer number of extra bytes to allocate."
  [x]
  (ByteBuffer/allocate (byte-size x)))

(defn write
  "Creates a new buffer, writes the given data structure to it, and returns
  that ByteBuffer, flipped."
  [x]
  (->> (buffer-for x)
       (write! x)
       .flip))
