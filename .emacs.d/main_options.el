(set-language-environment "UTF-8")

;; gnu k&r bsd stroustrup linux python java whitesmith ellemtel awk
(setq c-default-style "k&r")

(when (equal "" user-mail-address)
  (setq user-mail-address "melvin.laplanche+dev@gmail.com")
  )

(ido-mode 1)
(setq compile-command "make")
(blink-cursor-mode 0)

(column-number-mode 1)
(line-number-mode 1)
; (tool-bar-mode -1)
; (scroll-bar-mode -1)
(menu-bar-mode -1)

(setq indent-tabs-mode nil)
(setq standard-indent 2)
(setq tab-width 8)
(setq c-basic-offset 2)

(setq scroll-step 1)
(setq visible-bell nil)
(fset 'yes-or-no-p 'y-or-n-p)
(setq make-backup-files nil)
(setq require-final-newline t)
(add-hook 'write-file-hooks 'delete-trailing-whitespace)
(setq inhibit-startup-message t)

;; Complétion automatique
(abbrev-mode t)

;; Pas d'autosaves, et garder les backups dans un dossier à part
(setq auto-save-default nil)
(defvar backup-dir "~/.emacs.d/backups/")
(setq backup-directory-alist (list (cons "." backup-dir)))

;; la roue de la souris fait défiler beaucoup trop vite
(setq mouse-wheel-scroll-amount '(1))
(setq mouse-wheel-progressive-speed nil)

;; Colorisation syntaxique maximale dans tous les modes
(require 'font-lock)
(global-font-lock-mode t)
(setq font-lock-maximum-decoration t)
(setq font-lock-maximum-size nil)

;; Toujours montrer la correspondance des parenthèses
(require 'paren)
(show-paren-mode t)
(setq blink-matching-paren t)
(setq blink-matching-paren-on-screen t)
(setq show-paren-style 'expression)
(setq blink-matching-paren-dont-ignore-comments t)

;; On retire le troncage des lignes
; (setq truncate-partial-width-windows nil)


;; Lors d'un « copier-coller » à la souris, insérer le texte au niveau
;; du point cliqué et non à la position du curseur texte.
;(setq mouse-yank-at-point nil)

;; Facilite la recherche de buffer (C-x b)
;(iswitchb-mode t)

(ide-mode)
