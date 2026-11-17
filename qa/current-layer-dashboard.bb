#!/usr/bin/env bb

(ns current-layer-dashboard
  (:require [babashka.fs :as fs]
            [org.httpkit.server :as http]))

(defn -main [script root]
  (load-file script)
  (let [handler ((resolve 'pack-web/http-handler) (str (fs/canonicalize root)))
        server (http/run-server handler
                                {:ip "127.0.0.1"
                                 :port 0
                                 :worker-count 8
                                 :legacy-return-value? false})]
    (println (str "http://127.0.0.1:" (http/server-port server)))
    (flush)
    @(promise)))

(apply -main *command-line-args*)
