;;; my-autoload.el ---

;; Copyright (C) 2011  Melvin Laplanche

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

(defun my-autoload(dir &optional recur)
  "Add all directories from dir into the load-path"
  (interactive)
  (setq dir (file-name-as-directory dir))
  (if (equal recur nil)
      (setq recur 0)
    (when (equal recur t)
      (setq recur -1)))
  (add-to-list 'load-path dir)
  (dolist (file (directory-files dir))
    (setq file-path (concat dir file))
    (when (file-directory-p file-path)
      (unless (or (equal file ".")
		  (equal file ".."))
	(if (equal recur 0)
	    (add-to-list 'load-path dir)
	  (my-autoload file-path (- recur 1)))
	))))

(provide 'my-autoload)
;;; my-autoload.el ends here
