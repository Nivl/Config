;;; hooks.el ---


; HTML5
;(defun my-nxml-mode-hook ()
;  (eval-after-load "rng-loc"
;    '(add-to-list 'rng-schema-locating-files
;		  "~/.emacs.d/autoloadable/html5-el/schemas.xml"))
;  (require 'whattf-dt))
;(add-hook 'nxml-mode-hook 'my-nxml-mode-hook)

; C/C++

;; Le module ctypes permet d'ajouter à la liste des types de données
;; C/C++ des types non reconnus par défaut. Ces types sont alors
;; connus et gérés par le module de colorisation syntaxique.
(defun my-c-mode-hook ()
  (require 'ctypes)
  (turn-on-font-lock))
;(defun my-ctypes-load-hook ()
;  (ctypes-read-file "~/.emacs.d/ctypes_std_c" nil t t))
(add-hook 'c-mode-hook 'my-c-mode-hook)
(add-hook 'c++-mode-hook 'my-c-mode-hook)
;(add-hook 'ctypes-load-hook 'my-ctypes-load-hook)

; Activation systématique du mode mineur HS dans les modes C/C++
;(add-hook 'c-mode-common-hook 'hs-minor-mode t)
