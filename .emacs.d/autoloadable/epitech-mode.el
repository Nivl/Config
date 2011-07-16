;;
;; epitech-mode.el for  in /home/laplan_m/.emacs.d/bin
;;
;; Made by melvin laplanche
;; Login   <laplan_m@epitech.net>
;;
;; Started on  Sat Jun 25 16:15:09 2011 melvin laplanche
;; Last update Sat Jun 25 16:16:32 2011 melvin laplanche
;;

(load "../std.elc")
(load "../std_comment.elc")

; Overide EPITECH shortcut
;(global-set-key "\C-h" 'describe-key-briefly)
(global-set-key	(kbd "C-c h") 'std-file-header)
;(global-set-key (kbd "C-c h") 'insert-protect-header)

(provide 'epitech-mode)
