#!/usr/bin/env bb

(ns doorbell
  (:require [babashka.fs :as fs]
            [babashka.process :as process]
            [clojure.edn :as edn]
            [clojure.string :as str]))

(def usage-text
  (str "Ring a chat request that was written down but never reached the role.\n"
       "\n"
       "Usage:\n"
       "  doorbell.sh [<forge-root>] [--idler <path>]\n"
       "\n"
       "The dashboard writes the operator's message down, types it into the master\n"
       "role's pane and answers 200 — and when the typing fails, only its own stderr\n"
       "says so. The bridge counts the work done, and the idler check examines only\n"
       "roles holding cards, so a request that never arrived is invisible everywhere.\n"
       "The doorbell is that repair, and it is visible rather than hoped for.\n"
       "\n"
       "For every pending request it says one of three things: already delivered and\n"
       "left alone (the pane shows the request's own id, or an earlier pass saw it\n"
       "delivered); never delivered and rung again (the pane does not show the id and\n"
       "the role is not mid-turn); or not rung because the role is busy — injecting\n"
       "into a role mid-turn is exactly the case that loses a request.\n"
       "\n"
       "It never rings twice for one request: what it has seen and what it has rung\n"
       "live in <forge-root>/.swarmforge/doorbell.edn.\n"))

(defn exit! [status message]
  (binding [*out* *err*]
    (println message))
  (System/exit status))

(defn flag-value [args flag]
  (second (drop-while #(not= flag %) args)))

(defn rows [path]
  (when (fs/regular-file? path)
    (->> (str/split-lines (slurp (str path)))
         (remove str/blank?)
         (map #(str/split % #"\t" -1))
         vec)))

(defn forge-root [args]
  (let [given (first (remove #(str/starts-with? % "--") args))
        root (fs/absolutize (or given (str (fs/absolutize "."))))]
    (when-not (fs/regular-file? (fs/path root ".swarmforge" "roles.tsv"))
      (exit! 2 (str "Not a forge root (no .swarmforge/roles.tsv): " root)))
    root))

;; The master row is the role the dashboard types chat requests into: the one
;; whose own row says it is the master. The doorbell rings the same pane the
;; dashboard was supposed to.
(defn master-row [root]
  (some #(when (= "master" (nth % 1 nil)) %) (rows (fs/path root ".swarmforge" "roles.tsv"))))

(defn tmux-socket [root]
  (let [file (fs/path root ".swarmforge" "tmux-socket")]
    (when (fs/regular-file? file) (not-empty (str/trim (slurp (str file)))))))

(defn tmux [socket & args]
  (try
    (let [{:keys [out exit]} (apply process/sh (concat ["tmux" "-S" socket] args))]
      (when (zero? exit) (str/trim out)))
    (catch Exception _ nil)))

(defn pane-text [socket pane]
  (tmux socket "capture-pane" "-p" "-t" pane))

(defn request-files [root]
  (let [dir (fs/path root ".swarmforge" "dashboard" "requests" "pending")]
    (if (fs/directory? dir)
      (->> (fs/list-dir dir)
           (filter #(str/ends-with? (fs/file-name %) ".request"))
           (sort-by fs/file-name)
           vec)
      [])))

;; A request file is header lines, a blank line, then the body — the shape the
;; dashboard writes and every reader of it takes apart.
(defn parse-request [file]
  (let [[headers body] (str/split (slurp (str file)) #"\n\n" 2)
        field (fn [name]
                (some (fn [line]
                        (when (str/starts-with? line (str name ": "))
                          (subs line (+ 2 (count name)))))
                      (str/split-lines headers)))]
    {:id (field "id") :body (str/trim (or body "")) :file (str file)}))

(defn pending-requests [root]
  (->> (request-files root)
       (map parse-request)
       (remove #(str/blank? (:id %)))
       vec))

(defn ledger-path [root]
  (fs/path root ".swarmforge" "doorbell.edn"))

(defn ledger [root]
  (let [path (ledger-path root)]
    (if (fs/regular-file? path)
      (try
        (let [stored (edn/read-string (slurp (str path)))]
          {:seen (set (:seen stored)) :rung (set (:rung stored))})
        (catch Exception _ {:seen #{} :rung #{}}))
      {:seen #{} :rung #{}})))

(defn save-ledger! [root seen rung]
  (let [path (ledger-path root)]
    (fs/create-dirs (fs/parent path))
    (spit (str path) (pr-str {:seen (vec (sort seen)) :rung (vec (sort rung))}))))

;; The check that knows what "working" means for each tool is the idler check;
;; the doorbell asks it rather than repeating its rules, and treats anything but
;; a role mid-turn as a window to ring into.
(defn idler-check [root idler]
  (let [command (or idler (str (fs/path (fs/parent *file*) "role_health.sh")))]
    (try
      (let [{:keys [out]} (process/sh command root "--forge-root" root)]
        out)
      (catch Exception _ ""))))

(defn verdict-of [report role]
  (->> (str/split-lines (str report))
       (map #(str/split % #"\s+"))
       (filter #(and (seq %) (= role (first %))))
       first
       second))

(defn busy? [root role idler]
  (= "working" (verdict-of (idler-check root idler) role)))

;; The dashboard types the id in as [<id>], which is the only delivery evidence
;; there is: a capture of the pane, and the ledger for what the capture can no
;; longer hold.
(defn delivered-evidence [id pane]
  (boolean (and pane (str/includes? pane (str "[" id "]")))))

(defn wake-text [id body]
  (if (str/includes? (or body "") "\n")
    (str "[" id "]\n" body)
    (str "[" id "] " body)))

(defn ring! [socket pane id body]
  (tmux socket "send-keys" "-t" pane "-l" (wake-text id body))
  (tmux socket "send-keys" "-t" pane "C-m")
  (tmux socket "send-keys" "-t" pane "C-j"))

(defn -main [& args]
  (when (some #{"--help" "-h"} args)
    (println usage-text)
    (System/exit 0))
  (let [root (forge-root args)
        idler (flag-value args "--idler")
        row (master-row root)
        role (first row)
        pane (nth row 3 nil)
        socket (tmux-socket root)
        pane-text (when (and socket pane) (pane-text socket pane))
        stored (ledger root)
        seen (atom (:seen stored))
        rung (atom (:rung stored))
        requests (pending-requests root)]
    (println (str "doorbell: read the pane " (or pane "-") " of the role " (or role "-")
                  " for " (str root)))
    (if (empty? requests)
      (println "nothing pending: no chat request is waiting to be delivered")
      (doseq [{:keys [id body]} requests]
        (let [quoted (str "\"" body "\"")]
          (cond
            (or (contains? @rung id) (contains? @seen id) (delivered-evidence id pane-text))
            (do (swap! seen conj id)
                (println (str "the chat request " quoted " was already delivered and left alone")))

            (nil? pane)
            (println (str "the chat request " quoted " was not rung: the role " (or role "-")
                          " has no pane to ring"))

            (busy? root role idler)
            (do (swap! seen conj id)
                (println (str "the chat request " quoted " was not rung because the role "
                              (or role "-") " was busy")))

            :else
            (do (ring! socket pane id body)
                (swap! seen conj id)
                (swap! rung conj id)
                (println (str "the chat request " quoted " was never delivered and rung into " pane)))))))
    (save-ledger! root @seen @rung)
    (System/exit 0)))

(when (= (str *file*) (System/getProperty "babashka.file"))
  (apply -main *command-line-args*))
