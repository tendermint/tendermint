(defproject jepsen.tendermint "0.2.0"
  :description "Jepsen tests for the Tendermint Byzantine consensus system"
  :url "http://github.com/jepsen-io/tendermint"
  :license {:name "Apache License, version 2.0"
            :url "https://www.apache.org/licenses/LICENSE-2.0"}
  :dependencies [[org.clojure/clojure "1.10.1"]
                 [typed.clj/runtime "1.0.12"]
                 [cheshire "5.10.0"]
                 [slingshot "0.12.2"]
                 [clj-http "3.10.3"]
                 [jepsen "0.2.1"]]
  :profiles {:dev {:dependencies [[typed.clj/checker "1.0.12"]]}}
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
  :plugins [[lein-cljfmt "0.7.0"]]
  :main jepsen.tendermint.cli
  :injections [(require 'clojure.core.typed)
               (clojure.core.typed/install)])
