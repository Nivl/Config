;;; hooks.el ---

;; python

(require 'cl)

(defadvice python-calculate-indentation (around continuation-with-dot)
  "Handle continuation lines that start with a dot and try to
line them up with a dot in the line they continue from."
  (unless
      (block 'found-dot
        (save-excursion
          (beginning-of-line)
          (when (and (python-continuation-line-p)
                     (looking-at "\\s-*\\."))
            (save-restriction
              (narrow-to-region (point)
                                (save-excursion
                                  (end-of-line -1)
                                  (python-beginning-of-statement)
                                  (point)))
              (let ((p -1))
                (while (/= p (point))
                  (setq p (point))
                  (when (looking-back "\\.")
                    (setq ad-return-value (1- (current-column)))
                    (return-from 'found-dot t))
                  (ignore-errors (backward-sexp))))))))
    ad-do-it))

(ad-activate 'python-calculate-indentation)

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
