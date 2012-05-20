;;; misc_functions.el ---

;; Copyright (C) 2010  Melvin Laplanche

;; Author: Melvin <melvin.laplanche+dev@gmail.com>
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

(defun fix-frame-horizontal-size (width)
  "Set the frame's size to 80 (or prefix arg WIDTH) columns wide."
  (interactive "P")
  (if window-system
      (set-frame-width (selected-frame) (or width 80))
    (error "Cannot resize frame horizontally: is a text terminal")))


(defun fix-window-horizontal-size (width)
  "Set the window's size to 80 (or prefix arg WIDTH) columns wide."
  (interactive "P")
  (enlarge-window (- (or width 80) (window-width)) 'horizontal))

(defun fix-horizontal-size (width)
  "Set the window's or frame's width to 80 (or prefix arg WIDTH)."
  (interactive "P")
  (condition-case nil
      (fix-window-horizontal-size width)
    (error
     (condition-case nil
	 (fix-frame-horizontal-size width)
       (error
	(error "Cannot resize window or frame horizontally"))))))

(defun duplicate-line ()
  "Duplicate a line"
  (interactive)
  (move-beginning-of-line nil)
  (setq beg (point))
  (move-end-of-line nil)
  (kill-ring-save beg (point))
  (newline)
  (yank))

(defun comment-line-c ()
  "Comment a line (C style)"
  (interactive)
  (move-beginning-of-line nil)
  (indent-for-tab-command)
  (if (equal (point) (line-end-position))
      (progn
	(insert "/*  */")
	(backward-char)
	(backward-char)
	(backward-char))
    (progn
      (insert "/* ")
      (move-end-of-line nil)
      (insert " */"))))

(defun buffer-mode ()
  "Returns the current major mode."
  (interactive)
  (message "%s" major-mode))

(defun get-filename ()
  "return the filename"
  (interactive)
  (file-name-nondirectory(buffer-file-name)))

(defun get-filename-no-ext ()
  "return the filename"
  (interactive)
  (file-name-sans-extension(file-name-nondirectory(buffer-file-name))))

(defun get-date()
  "Return the current date."
  (interactive)
  (format-time-string "%B %d %Y at %I:%M %p"))

(defun kill-backward-line ()
   "Kill backward from point to beginning of line"
   (interactive)
   (kill-line 0))


;; REPLACED BY A SNIPPET
(defun get-protect-header ()
  "Return a protection for header files."
  (setq str (file-name-sans-extension
	     (file-name-nondirectory (buffer-file-name))))
  (concat "#ifndef " (upcase str) "_H_\n"
	  "# define " (upcase str) "_H_\n"
	  "\n"
	  "#endif /* !" (upcase str) "_H_ */\n"))

;; REPLACED BY epitech-mode or headers.el
;(defun save-with-info()
;  "Insert your name, email, and the current date before saving."
;  (interactive)
;  (if (not (buffer-modified-p))
;      (message "(No changes need to be saved)")
;    (progn
;      (setq char_actuel (point))
;      (setq found 0)
;      (goto-line 1)
;      (if (re-search-forward "last\t" nil t)
;	  (progn
;	    (insert "Laplanche Melvin <melvin.laplanche+dev@gmail.com> ")
;	    (insert-date)
;	    (kill-line)
;	    (goto-line 1)))
;      (move-to-column 0)
;      (goto-char char_actuel)
;      (save-buffer))))


(provide 'misc_functions)
;;; misc_functions.el ends here
