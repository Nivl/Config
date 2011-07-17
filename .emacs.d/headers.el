;;; headers.el ---

;; Copyright (C) 2011  melvin laplanche

;; Author: Melvin Laplanche <melvin.laplanche+dev@gmail.com>
;; Keywords:

;; This program is free software; you can redistribute it and/or modify
;; it under the terms of the GNU General Public License as published by
;; the Free Software Foundation, either version 3 of the License, or
;; (at your option) any later version.

;; This program is distributed in the hope that it will be useful,
;; but WITHOUT ANY WARRANTY; without even the implied warranty of
;; MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
;; GNU General Public License for more details.

;; You should have received a copy of the GNU General Public License
;; along with this program.  If not, see <http://www.gnu.org/licenses/>.

;;; Commentary:

;;

;;; Code:

(global-set-key (kbd "C-c h") 'insert-header)
(setq write-file-hooks (cons 'update-header write-file-hooks))

(setq header-comment	"Comment:\t\t"
      header-project	"Project:\t\t"
      header-started-by	"Started by\t\t"
      header-email-beg	"<"
      header-email-end	">"
      header-started-on	"On\t\t\t"
      header-last-by	"Last updated by\t"
      header-last-on	"On\t\t\t")


(setq header-c-alist	'((cmd . "") (start . "/*") (content . "** ")
			  (end . "*/"))

      header-php-alist	'((cmd . "<?php") (start . "/*") (content . "** ")
			  (end . "*/"))

      header-sh-alist	'((cmd . "#!/bin/bash") (start . "")
			  (content . "## ")	(end . ""))

      header-perl-alist	'((cmd . "#!/usr/bin/perl -w") (start . "")
			  (content . "## ")		 (end . ""))

      header-makefile-alist	'((cmd . "") (start . "") (content . "## ")
				  (end . ""))

      header-python-alist	'((cmd . "") (start . "") (content . "# ")
				  (end . ""))

      header-lisp-alist	'((cmd . "") (start . "") (content . ";; ")
			  (end . ""))

      header-html-alist	'((cmd . "") (start . "<!--") (content . "-- ")
			  (end . "-->"))

      header-latex-alist '((cmd . "\\usepackage{verbatim}")
			   (start . "\\begin{comment}")
			   (content . "")
			   (end . "\\end{comment}"))
      )

(setq header-modes-alist '((sh-mode		. header-sh-alist)
			   (makefile-mode	. header-makefile-alist)
			   (cmake-mode		. header-makefile-alist)
			   (yaml-mode		. header-makefile-alist)
			   (awk-mode		. header-makefile-alist)
			   (c-mode		. header-c-alist)
			   (c++-mode		. header-c-alist)
			   (java-mode		. header-c-alist)
			   (css-mode		. header-c-alist)
			   (pov-mode		. header-c-alist)
			   (javascript-mode	. header-c-alist)
			   (cperl-mode		. header-perl-alist)
			   (fundamental-mode	. header-lisp-alist)
			   (text-mode		. header-lisp-alist)
			   (emacs-lisp-mode	. header-lisp-alist)
			   (lisp-mode		. header-lisp-alist)
			   (sgml-mode		. header-html-alist)
			   (nxml-mode		. header-html-alist)
			   (django-html-mode	. header-html-alist)
			   (django-mode		. header-python-alist)
			   (python-mode		. header-python-alist)
			   (html-helper-mode	. header-html-alist)
			   (php-mode		. header-php-alist)
			   (latex-mode		. header-latex-alist)
			   ))

(defun insert-header ()
  "Insert file header"
  (interactive)
  (setq header-type (assoc major-mode header-modes-alist))
  (if (equal header-type nil)
      (progn
	(message "%s not supported." major-mode)
	(setq cmd (read-from-minibuffer "command: "))
	(setq cs (read-from-minibuffer "Comment start: "))
	(setq cc (read-from-minibuffer "Comment content: "))
	(setq ce (read-from-minibuffer "Comment end: ")))
    (progn
      (setq header-type (cdr header-type))
      (setq cmd (cdr (assoc 'cmd (eval header-type))))
      (setq cs (cdr (assoc 'start (eval header-type))))
      (setq cc (cdr (assoc 'content (eval header-type))))
      (setq ce (cdr (assoc 'end (eval header-type))))
      ))
  (beginning-of-buffer)
  (setq commentary-line 1)
  (when (not (equal cmd ""))
    (insert cmd "\n")
    (setq commentary-line 2))
  (when (not (equal cs ""))
    (insert cs "\n")
    (setq commentary-line (+ 1 commentary-line)))

  (insert cc header-comment "\n")
  (insert cc "\n")
  (insert cc header-project "\n")

  (insert cc header-started-by (user-full-name) " ")
  (insert header-email-beg user-mail-address header-email-end "\n")
  (insert cc header-started-on)
  (insert-date)
  (insert "\n")

  (insert cc header-last-by (user-full-name) " ")
  (insert header-email-beg user-mail-address header-email-end "\n")
  (insert cc header-last-on)
  (insert-date)
  (insert "\n")

  (when (not (equal ce ""))
    (insert ce "\n"))
  (goto-line commentary-line)
  (end-of-line))

(defun update-header ()
  "Insert file header"
  (interactive)
  (save-excursion
    (beginning-of-buffer)
    (if (search-forward header-last-by nil t)
	(progn
	  (setq header-type (assoc major-mode header-modes-alist))
	  (if (equal header-type nil)
	      (setq cc (read-from-minibuffer "Comment content: "))
	    (setq cc (cdr (assoc 'content (eval (cdr header-type))))))
	  (beginning-of-line)
	  (kill-line)
	  (kill-line)
	  (kill-line)
	  (insert cc header-last-by (user-full-name) " ")
	  (insert header-email-beg user-mail-address header-email-end "\n")
	  (insert cc header-last-on)
	  (insert-date)
	  ))))

(provide 'headers)
;;; headers.el ends here
