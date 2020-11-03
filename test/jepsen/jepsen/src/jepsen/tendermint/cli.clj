(ns jepsen.tendermint.cli
  "Command line interface to Tendermint tests."
  (:require [clojure.pprint :refer [pprint]]
            [clojure.tools.cli :as tc]
            [jepsen [cli :as jc]]
            [jepsen.tendermint [core :as core]]))

(def opts
  "Extra command line opts."
  [[nil "--workload WORKLOAD" "Test workload to run; e.g. cas-register, set"
    :default :cas-register
    :parse-fn keyword]
   [nil "--nemesis NEMESIS" "Nemesis to use; e.g. clocks"
   :default :none
   :parse-fn keyword]
   [nil "--dup-validators" "Whether to have multiple validators share the same key."]
   [nil "--super-byzantine-validators" "Should byzantine validators have just shy of 2/3 the voting weight?"]
   (jc/package-opt "tendermint-url" "https://github.com/tendermint/tendermint/releases/download/v0.33.8/tendermint_v0.33.8_linux_amd64.zip")
   (jc/package-opt "merkleeyes-url" "")
   (jc/package-opt "abci-url" "")])


(defn -main
  [& args]
  (jc/run! (merge (jc/serve-cmd)
                  (jc/single-test-cmd {:test-fn core/test
                                       :opt-spec opts}))
          args))
