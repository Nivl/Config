
;; M-i = Tabulation
;; C-h c <keys> = Affiche le nom d’une commande appelée par les touches <keys>.
;; C-h k <keys> = Affiche le manuel de la commande appelée par <keys>.

; C-c h defined in epitech-mode.el and headers.el

; (global-unset-key (kbd "C-x C-z"))
(global-unset-key (kbd "C-x C-d"))
(global-unset-key (kbd "C-x C-o"))
(global-set-key (kbd "C-c d") 'duplicate-line)
(global-set-key (kbd "C-x C-n") 'next-buffer)
(global-set-key (kbd "C-x C-p") 'previous-buffer)
(global-set-key (kbd "C-c c") 'comment-line-c)
(global-set-key (kbd "M-p") 'backward-paragraph)
(global-set-key (kbd "M-n") 'forward-paragraph)
(global-set-key (kbd "M-k") 'kill-backward-line)
(global-set-key (kbd "M-d") 'delete-backward-char)
(global-set-key (kbd "M-r") 'backward-kill-word)
(global-set-key (kbd "C-r") 'kill-word)
(global-set-key (kbd "C-c k") 'kill-this-buffer)
(global-set-key (kbd "C-c r") 'replace-string)
(global-set-key (kbd "C-M-_") 'redo)
(global-set-key (kbd "C-c u") 'uncomment-region)
(global-set-key (kbd "M-g") 'goto-line)
(global-set-key (kbd "C-c n") 'insert-filename-no-ext)
(global-set-key (kbd "C-x C-b") 'buffer-menu)
(global-set-key (kbd "C-c g") 'gdb)
(global-set-key (kbd "C-c C-c") 'compile)
(global-set-key (kbd "C-c i") 'ide-mode-resize)
(global-set-key (kbd "C-c p") 'fix-horizontal-size)
