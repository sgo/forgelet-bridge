#!/usr/bin/env bb
;; The fixture's terminal: a pane with a composer.
;;
;; A role's pane is an agent's terminal, and what it draws is the turns it has
;; taken above the text it is still holding - the composer. This fixture is that
;; terminal, small enough to say what the doorbell has to read from one: a
;; bracketed paste arrives as one block, so a body of many lines cannot submit
;; itself on its own newlines; Enter takes the block as one turn and empties the
;; composer; and a pane told to lose the Enter leaves the text where it is,
;; which is what a terminal still taking a long paste does to the Enter that
;; follows it.
;;
;; Run it as a pane's command:
;;
;;   composer.bb [--lose-enter]
;;
;; Its tty is the pane's own, so the drawing lands where a session would read it:
;; every turn on a line of its own, and the composer under them with the cursor
;; at its end.
(ns composer
  (:require [babashka.process :as process]
            [clojure.string :as str]))

(def lose-enter? (boolean (some #{"--lose-enter"} *command-line-args*)))

;; The sequences tmux puts around a paste: bracketed paste is what an agent's
;; terminal asks for, and it is what keeps the pasted text out of the keymap.
(def paste-start [27 91 50 48 48 126])
(def paste-end [27 91 50 48 49 126])

(defn read-byte []
  (let [b (.read System/in)]
    (when (>= b 0) b)))

(defn bytes->text [bytes]
  (String. (byte-array (map byte bytes)) "UTF-8"))

;; A paste arrives with the terminal's own carriage returns for the newlines a
;; person would have typed: what the pane holds is the text, so it is put back
;; the way a reader reads it.
(defn pasted-text [bytes]
  (-> (bytes->text bytes)
      (str/replace "\r\n" "\n")
      (str/replace "\r" "\n")))

(defn prefix? [whole part]
  (= part (vec (take (count part) whole))))

(defn ends-with? [whole tail]
  (= tail (vec (take-last (count tail) whole))))

(defn drop-last-char [text]
  (subs text 0 (max 0 (dec (count text)))))

;; The frame a terminal draws: what it has taken, then what it is holding, with
;; the cursor left where the next character would go.
(defn draw! [turns composer]
  (print "\u001b[2J\u001b[H")
  (doseq [turn turns]
    (print "> ")
    (print (str/replace turn "\n" "\r\n"))
    (print "\r\n"))
  (print "\u001b[K› ")
  (print (str/replace composer "\n" "\r\n"))
  (flush))

(defn run []
  (process/sh ["sh" "-c" "stty raw -echo < /dev/tty"])
  (print "\u001b[?2004h")
  (draw! [] "")
  (loop [mode :typing, held [], turns [], composer ""]
    (let [b (read-byte)]
      (cond
        (nil? b)
        (do (print "\u001b[?2004l") (flush) (System/exit 0))

        ;; Inside a paste: its own newlines are text, and the block ends when the
        ;; terminal says it does.
        (= mode :paste)
        (let [held (conj held b)]
          (if (ends-with? held paste-end)
            (let [composer (str composer (pasted-text (drop-last (count paste-end) held)))]
              (draw! turns composer)
              (recur :typing [] turns composer))
            (recur :paste held turns composer)))

        ;; An escape sequence: the one that matters is the paste's own start.
        (= mode :escape)
        (let [held (conj held b)]
          (cond
            (ends-with? held paste-start) (recur :paste [] turns composer)
            (prefix? paste-start held) (recur :escape held turns composer)
            :else (recur :typing [] turns composer)))

        (= b 27)
        (recur :escape [b] turns composer)

        ;; Enter takes the block as one turn and leaves the composer empty - or
        ;; is lost, which leaves the turn exactly where it was.
        (or (= b 13) (= b 10))
        (if lose-enter?
          (recur :typing [] turns composer)
          (let [turns (conj turns composer)]
            (draw! turns "")
            (recur :typing [] turns "")))

        (= b 127)
        (let [composer (drop-last-char composer)]
          (draw! turns composer)
          (recur :typing [] turns composer))

        :else
        (let [composer (str composer (bytes->text [b]))]
          (draw! turns composer)
          (recur :typing [] turns composer))))))

(run)
