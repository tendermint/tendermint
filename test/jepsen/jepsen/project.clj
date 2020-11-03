(defproject jepsen.tendermint "0.1.0"
  :description "Jepsen tests for the Tendermint Byzantine consensus system"
  :url "http://github.com/jepsen-io/tendermint"
  :license {:name "Apache License, version 2.0"
            :url "https://www.apache.org/licenses/LICENSE-2.0"}
  :dependencies [[org.clojure/clojure "1.8.0"]
                 [org.clojure/core.typed "0.4.0"]
                 [cheshire "5.7.1"]
                 [slingshot "0.12.2"]
                 [clj-http "3.6.1"]
                 [jepsen "0.1.6"]]
  :jvm-opts ["-Xmx6g"
             "-XX:+UseConcMarkSweepGC"
             "-XX:+UseParNewGC"
             "-XX:+CMSParallelRemarkEnabled"
             "-XX:+AggressiveOpts"
             "-XX:+UseFastAccessorMethods"
             "-XX:MaxInlineLevel=32"
             "-XX:MaxRecursiveInlineLevel=2"
             "-XX:-OmitStackTraceInFastThrow"
             "-server"]
  :main jepsen.tendermint.cli
  :injections [(require 'clojure.core.typed)
               (clojure.core.typed/install)])
