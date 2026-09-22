#!/usr/bin/env bb

(ns route-card
  (:require [babashka.fs :as fs]
            [clojure.edn :as edn]
            [clojure.java.shell :as sh]
            [clojure.string :as str]))

(def usage-text
  (str "Route a new board card, gated on an operator approval.\n"
       "\n"
       "Usage:\n"
       "  route_card.sh propose <project-root> <task-name> <note-file>\n"
       "  route_card.sh commit <proposal-id> [--approval <record-id>]\n"
       "\n"
       "propose writes the proposal into the forge's dashboard clarifications\n"
       "and records it; commit creates the card only when the operator's own\n"
       "words in that record are affirmative. Each approval is single-use.\n"
       "\n"
       "Run both commands with the forge root as the working directory. The\n"
       "forge root defaults to $PWD and can be overridden with --forge-root.\n"
       "\n"
       "Approval forms: answering \"approve\" (or yes/ok/go ahead) is enough,\n"
       "because the answer is tied to this one proposal; replying with the\n"
       "card's name works too, and a negation anywhere in the answer refuses.\n"
       "Any other answer declines. --approval lets an approval left in the\n"
       "chat channel stand in for the clarification, and --operator-said\n"
       "records approval the operator gave in a channel that leaves no file\n"
       "(the pane). Either one passed to propose records the proposal without\n"
       "asking again in the dashboard.\n"))

(def affirmative-re #"(?i)\b(approve|approved|yes|ok|okay|go ahead|go)\b")

(def negative-re #"(?i)\b(no|not|dont|don't|won't|wont|wait|hold|reject|never)\b")

(defn usage []
  (println usage-text))

(defn exit! [status message]
  (binding [*out* *err*]
    (println message))
  (System/exit status))

(defn now []
  (.format java.time.format.DateTimeFormatter/ISO_INSTANT (java.time.Instant/now)))

(defn script-dir []
  (fs/parent *file*))

(defn flag-value [args flag]
  (second (drop-while #(not= flag %) args)))

(defn forge-root [args]
  (let [given (or (flag-value args "--forge-root")
                  (not-empty (System/getenv "SWARMFORGE_FORGE_ROOT"))
                  (str (fs/absolutize ".")))]
    (when-not (fs/regular-file? (fs/path given ".swarmforge" "roles.tsv"))
      (exit! 1 (str "Not a forge root (no .swarmforge/roles.tsv): " given)))
    (when-not (fs/directory? (fs/path given "projects"))
      (exit! 1 (str "Not a forge root (no projects/): " given)))
    (fs/absolutize given)))

(defn project-root [given]
  (let [root (fs/absolutize given)]
    (when-not (fs/directory? root)
      (exit! 1 (str "Project root not found: " root)))
    (when-not (fs/regular-file? (fs/path root ".swarmforge" "roles.tsv"))
      (exit! 1 (str "Not a project root (no .swarmforge/roles.tsv): " root)))
    root))

(defn parse-request [path]
  (let [[head body] (str/split (slurp (str path)) #"\n\n" 2)
        headers (into {}
                      (for [line (str/split-lines (or head ""))
                            :let [[k v] (str/split line #": " 2)]
                            :when (and k v)]
                        [k v]))]
    {:headers headers
     :body (or body "")
     :response (str/replace (get headers "response" "") #"\\n" "\n")}))

(defn clarification-path [root id state]
  (fs/path root ".swarmforge" "dashboard" "clarifications" state (str id ".request")))

(defn chat-path [root id state]
  (fs/path root ".swarmforge" "dashboard" "requests" state (str id ".request")))

(defn record-in-root
  "The operator's own words, from an answered clarification or from a chat
  request they wrote. Returns nil when the id is unknown in this root."
  [root id]
  (let [answered (clarification-path root id "done")
        pending (clarification-path root id "pending")
        chat-pending (chat-path root id "pending")
        chat-done (chat-path root id "done")]
    (cond
      (fs/regular-file? answered)
      (let [r (parse-request answered)]
        {:kind "clarification" :path (str answered) :text (:response r)})

      (fs/regular-file? chat-pending)
      (let [r (parse-request chat-pending)]
        {:kind "chat request" :path (str chat-pending) :text (:body r)})

      (fs/regular-file? chat-done)
      (let [r (parse-request chat-done)]
        {:kind "chat request" :path (str chat-done) :text (:body r)})

      (fs/regular-file? pending)
      {:kind "clarification" :path (str pending) :pending true :text ""}

      :else nil)))

;; Root order matters: a proposal's question is answered in the project store
;; (the dashboard aggregates open projects, so a question written at the forge
;; root would never surface), and the chat channel writes into the forge root.
(defn approval-record [roots id]
  (some #(record-in-root % id) roots))

(defn approval-from
  "Resolve the operator's words for a proposal: an explicit record id, words
  transcribed from a channel that leaves no file, a source recorded at propose
  time, or the proposal's own dashboard question."
  [roots proposal explicit-approval explicit-said]
  (let [from-record (fn [id]
                      (let [record (approval-record roots id)]
                        (cond
                          (nil? record)
                          (exit! 1 (str "No approval record '" id "' in "
                                        (str/join ", " (map str roots))
                                        "/.swarmforge/dashboard."))
                          (:pending record)
                          (exit! 1 (str "Approval '" id "' is still unanswered ("
                                        (:path record) "). The operator has not replied yet."))
                          :else (assoc record :via (str (:kind record) " " id)))))]
    (cond
      (not (str/blank? (or explicit-approval ""))) (from-record explicit-approval)
      (not (str/blank? (or explicit-said "")))
      {:kind "chat transcription" :text explicit-said :via "operator words in chat"}
      (not (str/blank? (or (:approval-id proposal) ""))) (from-record (:approval-id proposal))
      (not (str/blank? (or (:operator-said proposal) "")))
      {:kind "chat transcription" :text (:operator-said proposal) :via "operator words in chat"}
      :else (from-record (:id proposal)))))

(defn affirmative?
  "Consent from the operator's own words. A negation anywhere refuses, so
  \"no, not yet\" and \"don't approve\" cannot slip through on a stray \"ok\"."
  [text card-name]
  (let [text (or text "")]
    (cond
      (str/blank? text) false
      (re-find negative-re text) false
      (re-find affirmative-re text) true
      :else (and (not (str/blank? card-name))
                 (str/includes? (str/lower-case text) (str/lower-case card-name))))))

(defn proposal-file [forge id]
  (fs/path forge ".swarmforge" "route-proposals" (str id ".edn")))

(defn require-package [file ns-sym]
  (load-file (str (fs/path (script-dir) file)))
  ns-sym)

(defn master-lane [project]
  (let [result (sh/sh (str (fs/path (script-dir) "pack_board.sh"))
                      "master-lane" "--root" (str project))]
    (when-not (zero? (:exit result))
      (exit! 1 (str "Could not resolve the master lane for " project "\n"
                    (:err result) (:out result))))
    (str/trim (:out result))))

(defn board-task-id [project card-name]
  (let [file (fs/path project ".swarmforge" "board" "tasks.tsv")]
    (when (fs/regular-file? file)
      (some (fn [line]
              (let [cols (str/split line #"\t" -1)]
                (when (= (str/lower-case card-name) (str/lower-case (or (first cols) "")))
                  (nth cols 4 nil))))
            (str/split-lines (slurp (str file)))))))

(defn propose! [args]
  (let [forge (forge-root args)
        project (project-root (first args))
        name (second args)
        note-file (nth args 2 nil)
        approval-id (flag-value args "--approval")
        operator-said (flag-value args "--operator-said")
        pre-approved (or (not (str/blank? (or approval-id "")))
                         (not (str/blank? (or operator-said ""))))]
    (when (str/blank? name)
      (exit! 1 "Missing card name"))
    (when-not (and note-file (fs/regular-file? note-file))
      (exit! 1 (str "Note file not found: " note-file)))
    (when pre-approved
      (let [words (:text (approval-from [project forge] {} approval-id operator-said))]
        (when-not (affirmative? words name)
          (exit! 1 (str "Not approved: \"" (str/trim (first (str/split-lines (str/trim (or words "")))))
                        "\" — nothing recorded.")))))
    (let [note (str/trimr (slurp (str note-file)))
          lane (master-lane project)
          role (or (not-empty (System/getenv "SWARMFORGE_ROLE")) "lieutenant")
          body (str "Route a new card?\n\n"
                    "Project: " (fs/file-name project) "\n"
                    "Card: " name "\n"
                    "Lane: " lane "\n\n"
                    note "\n\n"
                    "Reply \"approve\" (or the card name) to approve; any other answer declines.")
          ;; With an approval already in hand the question would be redundant,
          ;; so the proposal is recorded under a timestamp id instead.
          id (if pre-approved
               (str "proposal-" (str/replace (now) #"[^0-9A-Za-z]" ""))
               ;; The question goes into the project's store: the forge
               ;; dashboard aggregates clarifications from open projects, so a
               ;; question left at the forge root would never be shown.
               (do (require-package "pack_dashboard_request.bb" 'pack-dashboard-request)
                   ((requiring-resolve 'pack-dashboard-request/create-clarification!)
                    (str project) role body)))
          record {:id id :project (str project) :name name :note note
                  :lane lane :created-at (now)
                  :approval-id approval-id
                  :operator-said operator-said}]
      (fs/create-dirs (fs/parent (proposal-file forge id)))
      (spit (str (proposal-file forge id)) (pr-str record))
      (println "PROPOSED:" id "card" name "in" (str project) "lane" lane)
      (if pre-approved
        (println "Approval already in hand; run: route_card.sh commit" id)
        (do (println "Awaiting the operator's answer in the dashboard.")
            (println "Then run: route_card.sh commit" id)))
      id)))

(defn commit! [args]
  (let [forge (forge-root args)
        id (first args)
        explicit (flag-value args "--approval")
        said (flag-value args "--operator-said")]
    (when (str/blank? id)
      (exit! 1 "Missing proposal id"))
    (let [file (proposal-file forge id)]
      (when-not (fs/regular-file? file)
        (exit! 1 (str "No proposal record for " id " at " file)))
      (let [proposal (edn/read-string (slurp (str file)))
            {:keys [project name note lane consumed-at task-id]} proposal
            record (approval-from [(fs/absolutize project) forge] proposal explicit said)]
        (when consumed-at
          (exit! 1 (str "Approval " id " was already used to create " task-id " — nothing created.")))
        (when-not (affirmative? (:text record) name)
          (exit! 1 (str "Not approved: " (:via record) " reads \""
                        (str/trim (first (str/split-lines (str/trim (or (:text record) "")))))
                        "\" — nothing created. Re-propose with route_card.sh propose.")))
        (let [current-lane (master-lane project)
              _ (when (not= current-lane lane)
                  (binding [*out* *err*]
                    (println (str "Note: lane moved from '" lane "' to '" current-lane
                                  "' since the proposal; the card will land in '" current-lane "'."))))
              _ (require-package "pack_web.bb" 'pack-web)
              create-task! (requiring-resolve 'pack-web/create-task!)]
          (try
            (create-task! project name note)
            (catch Exception e
              (exit! 1 (str "Card creation failed: " (.getMessage e)))))
          (let [created-id (board-task-id project name)]
            (spit (str file) (pr-str (assoc (edn/read-string (slurp (str file)))
                                            :consumed-at (now)
                                            :approved-by (:via record)
                                            :approved-text (str/trim (or (:text record) ""))
                                            :task-id created-id)))
            (println "APPROVED:" (:via record))
            (println "CREATED:" name "in lane" current-lane
                     (when created-id (str "task-id " created-id)))
            (println "The New Task note is in the project's outbox for the handoff daemon.")
            created-id))))))

(defn -main [& args]
  (when (some #{"--help" "-h"} args)
    (usage)
    (System/exit 0))
  (case (first args)
    "propose" (propose! (rest args))
    "commit" (commit! (rest args))
    (do (usage)
        (System/exit 1)))
  (System/exit 0))

(when (= (str *file*) (System/getProperty "babashka.file"))
  (apply -main *command-line-args*))
