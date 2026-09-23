#!/usr/bin/env bb

(ns role-health
  (:require [babashka.fs :as fs]
            [babashka.process :as process]
            [clojure.edn :as edn]
            [clojure.string :as str]))

(def usage-text
  (str "Report whether each role holding a card is working, waiting or stalled.\n"
       "\n"
       "Usage:\n"
       "  role_health.sh <project-root> [--forge-root <root>] [--watch [<seconds>]]\n"
       "                  [--ask] [--notify] [--json]\n"
       "\n"
       "Detection only. Nothing is resumed: --ask puts one question into a stalled\n"
       "role's pane, once per stall, and the role answers by continuing or by\n"
       "raising a clarification the operator can decide. Automatic nudging is\n"
       "deliberately absent — a nudge gives the session no new information, so it\n"
       "stalls again or starts work nobody asked for.\n"
       "\n"
       "--notify raises a chat request naming the stall, so it reaches the operator\n"
       "through the bridge's chat room the same way any forge question does. It is\n"
       "once per stall, and it is a question to the operator, never a nudge to the\n"
       "role.\n"
       "\n"
       "Verdicts: working; waiting on a decision or on the operator; waiting for\n"
       "pickup (mail in new/); idle holding a card; quiet between turns; assigned\n"
       "but not yet handed over; idle with nothing assigned; session gone; a tool\n"
       "the check does not know; or a forge with no role session up.\n"
       "Exit status is non-zero when any role is stalled (waiting for pickup, idle\n"
       "holding a card, or session gone).\n"))

(def grace-minutes 3)
(def ask-cooldown-minutes 30)

(defn exit! [status message]
  (binding [*out* *err*]
    (println message))
  (System/exit status))

(defn now [] (java.time.Instant/now))

(defn flag-value [args flag]
  (second (drop-while #(not= flag %) args)))

(defn flag? [args flag] (boolean (some #{flag} args)))

(defn project-root [args]
  (let [given (first (remove #(str/starts-with? % "--") args))
        root (when given (fs/absolutize given))]
    (when-not (and root (fs/regular-file? (fs/path root ".swarmforge" "roles.tsv")))
      (exit! 2 "Give a project root: role_health.sh <project-root> [--ask]"))
    root))

(defn forge-root [args project]
  (let [given (flag-value args "--forge-root")
        root (fs/absolutize (or given (fs/parent (fs/parent project))))]
    (when-not (fs/directory? (fs/path root "projects"))
      (exit! 2 (str "Not a forge root (no projects/): " root)))
    root))

(defn rows [path]
  (when (fs/regular-file? path)
    (->> (str/split-lines (slurp (str path)))
         (remove str/blank?)
         (map #(str/split % #"\t" -1))
         vec)))

;; roles.tsv: role, branch, worktree, pane, display name, tool, mode, flow
(defn roles [project]
  (->> (rows (fs/path project ".swarmforge" "roles.tsv"))
       (map (fn [[role branch worktree pane display tool & _]]
              {:role role :branch branch :worktree worktree :pane pane
               :display display :tool (or tool "codex")}))
       vec))

;; A forge does not run one agent: the roles file records a tool per role, and
;; Saibill mixes codex and claude — its lieutenant is claude. Two lessons from
;; reading those sessions rather than assuming: a pane's command is not the
;; agent's name (Claude Code's shows as its version, e.g. 2.1.277), so "is the
;; session alive" is decided by the pane having a foreground process that is not a
;; shell; and the reliable sign of work is the agent's own transcript, because the
;; marker a terminal draws is that agent's wording and changes between versions.
;; An agent we do not know yet is reported as such rather than judged idle — a
;; false stall on a healthy session is the failure worth avoiding.
(def shells #{"zsh" "bash" "sh" "dash" "ksh" "fish" "tcsh" "csh"})

(defn alive? [command]
  (and (some? command)
       (not (contains? shells (str/lower-case command)))))

(defn known-tool? [tool]
  (contains? #{"codex" "claude"} (str/lower-case (or tool ""))))

;; Claude Code keeps one transcript directory per working directory, the path
;; with its separators turned into dashes: /Users/sgo/sgo -> -Users-sgo-sgo.
(defn claude-transcript-dir [worktree]
  (let [dashed (str/replace (str/replace worktree #"/" "-") #"_" "-")]
    (fs/path (or (not-empty (System/getenv "ROLE_HEALTH_CLAUDE_PROJECTS"))
                 (str (fs/path (System/getProperty "user.home") ".claude" "projects")))
             dashed)))

(defn newest-millis-in [dir]
  (when (fs/directory? dir)
    (->> (file-seq (fs/file dir))
         (remove fs/directory?)
         (filter #(str/ends-with? (str %) ".jsonl"))
         (map #(.toMillis (fs/last-modified-time %)))
         (sort)
         (last))))

;; Working, per agent: codex names its state in the pane ("esc to interrupt"),
;; claude records it in its transcript, so a write within the last minute means it
;; is working whatever its pane happens to draw.
(defn working? [tool worktree text]
  (case (str/lower-case (or tool "codex"))
    "codex" (boolean (and text (str/includes? (str/lower-case text) "esc to interrupt")))
    "claude" (boolean (let [newest (newest-millis-in (claude-transcript-dir worktree))]
                        (and newest (> newest (- (System/currentTimeMillis) 60000)))))
    nil))

;; tasks.tsv: name, lane, created, updated, task-id, audit count
(defn cards-by-lane [project]
  (reduce (fn [acc [name lane & _]]
            (if (and name lane (not= lane "done"))
              (update acc lane (fnil conj []) name)
              acc))
          {}
          (rows (fs/path project ".swarmforge" "board" "tasks.tsv"))))

(defn tmux-socket [project]
  (let [path (fs/path project ".swarmforge" "tmux-socket")]
    (when (fs/regular-file? path) (str/trim (slurp (str path))))))

(defn tmux [socket & args]
  (try
    (let [{:keys [out exit]} (apply process/sh
                                    (concat ["tmux" "-S" socket] args))]
      (when (zero? exit) (str/trim out)))
    (catch Exception _ nil)))

(defn pane-command [socket pane]
  (tmux socket "display" "-p" "-t" pane "#{pane_current_command}"))

(defn pane-text [socket pane]
  (tmux socket "capture-pane" "-p" "-t" pane))

(defn inbox-count [worktree state]
  (let [dir (fs/path worktree ".swarmforge" "handoffs" "inbox" state)]
    (if (fs/directory? dir)
      (count (remove #(str/starts-with? (fs/file-name %) ".") (fs/list-dir dir)))
      0)))

;; What says whether a card was handed over is the role's own inbox, not the
;; board: mail in `new` is waiting for pickup, and a note already in `in_process`
;; is work the role took up and is holding. A card in the lane whose note has not
;; arrived is queued for later, which is not a stall.

;; A role waiting for the operator to approve a handoff is waiting on the one
;; thing this check must never raise an alarm about: the operator's own pace.
(defn approval-waiting-on-the-operator? [project role-name]
  (let [dir (fs/path project ".swarmforge" "handoffs" "pending_approval")]
    (boolean
     (when (fs/directory? dir)
       (some (fn [file]
               (let [text (slurp (str file))]
                 (boolean (re-find (re-pattern (str "(?m)^from: " (java.util.regex.Pattern/quote role-name) "\\s*$")) text))))
             (remove fs/directory? (fs/list-dir dir)))))))

;; A role with an open clarification is waiting on a decision, not stopped. The
;; check reads the project's own store, where each request names the role it came
;; from.
(defn waiting-on-a-decision? [project role]
  (let [dir (fs/path project ".swarmforge" "dashboard" "clarifications" "pending")]
    (boolean
     (when (fs/directory? dir)
       (some (fn [file]
               (let [text (slurp (str file))]
                 (boolean (re-find (re-pattern (str "(?m)^role: " (java.util.regex.Pattern/quote role) "\\s*$")) text))))
             (fs/list-dir dir))))))

(defn newest-mtime [dir]
  (when (fs/directory? dir)
    (->> (file-seq (fs/file dir))
         (remove fs/directory?)
         (map #(.toMillis (fs/last-modified-time %)))
         (sort)
         (last))))

(defn last-commit-time [worktree]
  (try
    (let [{:keys [out exit]} (process/sh "git" "-C" worktree "log" "-1" "--format=%ct")]
      (when (zero? exit) (some-> (str/trim out) not-empty Long/parseLong (* 1000))))
    (catch Exception _ nil)))

(defn quiet-minutes [role]
  (let [marks (remove nil? [(newest-mtime (fs/path (:worktree role) "tmp"))
                            (last-commit-time (:worktree role))])]
    (when (seq marks)
      (quot (- (System/currentTimeMillis) (apply max marks)) (* 1000 60)))))

(defn verdict-for [socket project role lane-cards]
  (let [command (pane-command socket (:pane role))
        text (pane-text socket (:pane role))
        tool (:tool role)
        new (inbox-count (:worktree role) "new")
        in-process (inbox-count (:worktree role) "in_process")
        quiet (quiet-minutes role)
        holding (boolean (or (seq lane-cards) (pos? in-process)))
        waiting (waiting-on-a-decision? project (:role role))
        awaiting-operator (approval-waiting-on-the-operator? project (:role role))]
    {:role (:role role)
     :pane (:pane role)
     :tool tool
     :command command
     :card (str/join ", " lane-cards)
     :new new
     :in_process in-process
     :quiet quiet
     :verdict
     (cond
       (not (alive? command)) :session-gone
       (not (known-tool? tool)) :tool-not-known
       (working? tool (:worktree role) text) :working
       ;; Before anything about quiet mail: if this role's handoff is sitting with
       ;; the operator, its queue is not stalled, it is blocked on a person.
       awaiting-operator :waiting-on-the-operator
       waiting :waiting-on-a-decision
       ;; Mail that arrived while the role was finishing a turn is not a stall
       ;; yet: the same grace that keeps a between-turns pause quiet applies to it.
       (and (pos? new) (or (nil? quiet) (> quiet grace-minutes))) :waiting-for-pickup
       (pos? new) :mail-just-arrived
       ;; A card in the lane that was never handed over is queued for later, not
       ;; a stall; a card the role took up and then went quiet on is.
       (and holding (not (pos? in-process)) (not (pos? new))) :assigned-not-taken
       (and holding (or (nil? quiet) (> quiet grace-minutes))) :idle-holding-card
       holding :quiet-between-turns
       :else :idle-nothing-assigned)}))

(def stalled? #{:waiting-for-pickup :idle-holding-card :session-gone})

;; A forge that is not running at all is not a stall: at login the agents do not
;; exist yet, and a watcher that speaks then cries wolf every morning. The forge
;; is up when any of its roles has a session; a role missing while others are
;; there is the stall worth reporting.
(defn forge-up? [socket roles]
  (boolean (some #(pane-command socket (:pane %)) roles)))

(defn report-line [{:keys [role verdict card new in_process quiet tool command] :as r}]
  (if (= verdict :forge-not-running)
    (str (format "%-11s %-24s %s" "forge" "not-running" "no role session is up")
         (if (str/blank? card) "" (str " (cards waiting: " card ")")))
    (str (format "%-11s %-24s %-34s new=%d in_process=%d quiet=%s tool=%s"
                 role
                 (name verdict)
                 (if (str/blank? card) "-" card)
                 new in_process
                 (if quiet (str quiet "m") "?")
                 (or tool "?"))
         ;; A session judged dead because its pane runs something else should say
         ;; what it found: that is a mismatch to read, not silence to guess at.
         (if (= verdict :session-gone)
           (str " (pane runs " (or command "nothing") ")")
           ""))))

(defn ask-text [{:keys [role card verdict quiet] :as r}]
  (str "[" (now) " role-health] Your session has been at a prompt for "
       (if quiet (str quiet " minutes") "a while")
       (when-not (str/blank? card) (str " while holding " card))
       ". What blocks you? Continue if you can. If you cannot, raise a clarification "
       "naming the card, what stops you, what you already tried, and what you need — "
       "do not start unrelated work."))

(defn ask-once! [state-path {:keys [role card verdict] :as r}]
  (let [state (if (fs/regular-file? state-path)
                (try (edn/read-string (slurp (str state-path))) (catch Exception _ {}))
                {})
        key (str role "|" (if (str/blank? card) "-" card) "|" (name verdict))
        last (get state key)
        fresh? (or (nil? last)
                   (> (- (System/currentTimeMillis) (* 1000 last))
                      (* 60 ask-cooldown-minutes 1000)))]
    (when fresh?
      (fs/create-dirs (fs/parent state-path))
      (spit (str state-path) (pr-str (assoc state key (quot (System/currentTimeMillis) 1000))))
      true)))

(defn inject! [socket pane text]
  (tmux socket "send-keys" "-t" pane "-l" text)
  (Thread/sleep 150)
  (tmux socket "send-keys" "-t" pane "C-m")
  (Thread/sleep 50)
  (tmux socket "send-keys" "-t" pane "C-j"))

(defn notify-text [forge {:keys [card verdict quiet role] :as r}]
  (str "Stall watch: " role " has been " (name verdict)
       (when-not (str/blank? card) (str " on " card))
       (when quiet (str " for " quiet " minutes"))
       ", and the work is not moving. The forge is " (fs/file-name forge)
       " (this watch covers several). Nothing has been nudged — decide whether to "
       "ask it what blocks it, send it back, or leave it."))

;; A chat request, in the shape the forge's dashboard writes for the operator's
;; own messages, so the bridge carries it to the phone the way it carries any
;; request. The answer is the lieutenant's, and the body says who is asking.
(defn raise-chat-request! [forge text]
  (let [id (str "req-"
                (-> (java.time.format.DateTimeFormatter/ofPattern "yyyyMMdd'T'HHmmss.SSSSSS")
                    (.withZone (java.time.ZoneOffset/UTC))
                    (.format (java.time.Instant/now))))
        dir (fs/path forge ".swarmforge" "dashboard" "requests" "pending")
        file (fs/path dir (str id ".request"))]
    (fs/create-dirs dir)
    (spit (str file)
          (str "id: " id "\n"
               "status: pending\n"
               "created_at: " (java.time.Instant/now) "\n"
               "\n"
               text "\n"))
    (str file)))

(defn survey [project forge ask? notify?]
  (let [socket (tmux-socket project)
        lanes (cards-by-lane project)
        state-path (fs/path forge ".swarmforge" "role-health.edn")
        role-rows (roles project)
        results (if (forge-up? socket role-rows)
                  (mapv #(verdict-for socket project % (get lanes (:role %))) role-rows)
                  [{:role "-" :pane "-" :card (str/join ", " (mapcat val lanes))
                    :new 0 :in_process 0 :quiet nil :verdict :forge-not-running}])]
    (doseq [r results]
      (when (and ask? (stalled? (:verdict r)))
        (when (ask-once! state-path r)
          (inject! socket (:pane r) (ask-text r))
          (println (str "ASKED " (:role r) ": " (name (:verdict r)))))))
    (doseq [r results]
      (when (and notify? (stalled? (:verdict r)))
        (when (ask-once! state-path (assoc r :verdict (keyword (str "notified-" (name (:verdict r))))))
          (raise-chat-request! forge (notify-text forge r))
          (println (str "NOTIFIED " (:role r) ": " (name (:verdict r)))))))
    results))

(defn -main [& args]
  (when (some #{"--help" "-h"} args)
    (println usage-text)
    (System/exit 0))
  (let [project (project-root args)
        forge (forge-root args project)
        watch? (flag? args "--watch")
        seconds (or (some-> (flag-value args "--watch") Long/parseLong) 15)
        ask? (flag? args "--ask")
        notify? (flag? args "--notify")]
    (loop [previous nil]
      (let [results (survey project forge ask? notify?)
            lines (mapv report-line results)
            stalled (filter #(stalled? (:verdict %)) results)]
        (when (not= lines previous)
          (doseq [line lines] (println line))
          (when (seq stalled)
            (println (str "STALLED: " (str/join ", " (map #(str (:role %) " (" (name (:verdict %)) ")") stalled))))
          (flush))
        (if watch?
          (do (Thread/sleep (* 1000 seconds))
              (recur lines))
          (System/exit (if (seq stalled) 1 0))))))))

(when (= (str *file*) (System/getProperty "babashka.file"))
  (apply -main *command-line-args*))
