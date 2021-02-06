;; enable package manager
(require 'package)
(add-to-list 'package-archives
             '("melpa-stable" . "https://stable.melpa.org/packages/"))
(package-initialize)


;; Display the column number in the mode bar
(setq column-number-mode t)

;; backups (tilde files)
(setq backup-directory-alist `(("." . "~/.emacs-saves")))
(setq version-control t     ;; Use version numbers for backups.
      kept-new-versions 10  ;; Number of newest versions to keep.
      kept-old-versions 2   ;; Number of oldest versions to keep.
      delete-old-versions t ;; Don't ask to delete excess backup versions.
      backup-by-copying t)  ;; Copy all files, don't rename them.
(setq vc-make-backup-files t)


;; flow-type
;; to emable on file having //@flow AND a .flowconfig
(add-hook 'js2-mode-hook 'flow-minor-enable-automatically)
;; to enable on all JS files
;; (add-hook 'js2-mode-hook 'flow-minor-mode)

(custom-set-variables
 ;; custom-set-variables was added by Custom.
 ;; If you edit it by hand, you could mess it up, so be careful.
 ;; Your init file should contain only one such instance.
 ;; If there is more than one, they won't work right.
 '(package-selected-packages
   (quote
    (flow-minor-mode typescript-mode flycheck-gometalinter flycheck go-mode markdown-mode))))
(custom-set-faces
 ;; custom-set-faces was added by Custom.
 ;; If you edit it by hand, you could mess it up, so be careful.
 ;; Your init file should contain only one such instance.
 ;; If there is more than one, they won't work right.
 )
