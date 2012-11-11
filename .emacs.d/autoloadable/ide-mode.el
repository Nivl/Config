;;; ide-mode.el ---

;; Copyright (C) 2010  Melvin

;; Author: Melvin <nivl@barney>
;; Keywords:

;; This program is free software; you can redistribute it and/or modify
;; it under the terms of the GNU General Public License as published by
;; the Free Software Foundation, either version 3 of the License, or
;; (at your option) any later version.

;; This program is distributed in the hope that it will be useful,
;; but WITHOUT ANY WARRANTY; without even the implied warranty of
;; MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
;; GNU General Public License for more details.

(defvar ide-mode-var-source-type "c" "Source type (c, cpp, py, …)")
(defvar ide-mode-var-buffer "*messages*" "Current buffer")
(defvar ide-mode-var-buffer-bot "*scratch*" "Buffer on the bottom")
(defvar ide-mode-var-build-path "./" "Path to the build directory/file")
(defvar ide-mode-var-build-from-cmake nil "We’re using cmake")

(defconst ide-mode-const-build-search-count 5 "Number of dir to browse.")
(defconst ide-mode-const-build-cmake ".." "Path from the build dir to cmake")
(defconst ide-mode-var-launched nil "Checks if the mode has been launched.")

(defun ide-mode-resize (&optional fix)
  "Resize the current buffer to 25 lines"
  (interactive)
  (if fix
      (setq nb-line 26)
    (setq nb-line 26))
  (when (not (equal nb-line (window-height)))
    (setq diff (- nb-line (window-height)))
    (enlarge-window (* diff 1))))

(defun ide-search-build-dir ()
  (setq path-cmake  "./build/")
  (setq path-makefile "./")
  (setq count 0)
  (while (and
	  (not (file-exists-p path-cmake))
	  (and (not (file-exists-p path-cmake))
	       (not (file-exists-p (concat path-makefile "Makefile"))))
	  (< count ide-mode-const-build-search-count))
    (setq path-makefile (concat "../" path-makefile))
    (setq path-cmake (concat "../" path-cmake))
    (setq count (+ 1 count)))
  (if (and (not (file-exists-p path-cmake))
	   (not (file-exists-p (concat path-makefile "Makefile"))))
      (error "No Makefile or build directory has been found.")
    (progn
      (if (not (file-exists-p path-cmake))
	  (setq ide-mode-var-build-path path-makefile)
	(progn
	  (setq ide-mode-var-build-path path-cmake)
	  (setq ide-mode-var-build-from-cmake 1))))))

(defun ide-mode-describe-key ()
  "Function used to describe a key"
  (interactive)
  (describe-key)
  (setq ide-mode-var-buffer-bot "*help*")
  (ide-mode-resize))

(defun ide-mode-eshell (&optional do-not-switch)
  "Function used to launch the eshell"
  (interactive)
  (if (and
       (equal "*eshell*" (buffer-name))
       (equal "*eshell*" ide-mode-var-buffer-bot))
      (when (not do-not-switch)
	(switch-to-buffer-other-window ide-mode-var-buffer))
    (progn
      (when (not (equal ide-mode-var-buffer-bot (buffer-name)))
	(ide-mode-resize)
	(setq ide-mode-var-buffer (buffer-name))
	(switch-to-buffer-other-window ide-mode-var-buffer-bot))
      (setq ide-mode-var-buffer-bot "*eshell*")
      (eshell))))

(defun ide-mode-compile ()
  "Function used to compile"
  (interactive)
  (setq ide-mode-var-source-type (file-name-extension (buffer-name)))
  (if (equal ide-mode-var-source-type "py")
      (ide-mode-validate-python)
    (ide-mode-compile-xmake)))

(defun ide-mode-validate-python ()
  "Function used to validate a python project"
  (interactive)
  (if (equal ide-mode-var-buffer-bot (buffer-name))
      (switch-to-buffer-other-window ide-mode-var-buffer))
  (pep8)
  (ide-mode-resize))

(defun ide-mode-compile-xmake ()
  "Function used to compile using cmake or a makefile"
  (interactive)
  (if (equal ide-mode-var-buffer-bot (buffer-name))
      (switch-to-buffer-other-window ide-mode-var-buffer))
  (ide-search-build-dir)
  (setq compile-opt (concat "cd " ide-mode-var-build-path))
  ;(error ide-mode-var-build-path)
  (if ide-mode-var-build-from-cmake
      (setq compile-opt (concat compile-opt
				" && cmake " ide-mode-const-build-cmake)))
  (setq compile-opt (concat compile-opt " && make -j4"))
  (compile compile-opt)
  (setq ide-mode-var-buffer (buffer-name))
  (setq ide-mode-var-buffer-bot "*compilation*")
  (switch-to-buffer-other-window "*compilation*")
  (end-of-buffer)
  (switch-to-buffer-other-window ide-mode-var-buffer)
  (ide-mode-resize))

(defun ide-mode ()
  "Mode to use emacs as ide"
  (interactive)
  (when (not ide-mode-var-launched)
    (global-set-key [remap compile] 'ide-mode-compile)
    (global-set-key [remap eshell] 'ide-mode-eshell)
;    (when (not (equal (one-window-p) nil))
;      (split-window-vertically)
;      (split-window-vertically))
;    (ide-mode-resize t)
;    (other-window 1)
;    (ide-mode-resize t)
;    (other-window 2)
    )
  (setq ide-mode-var-launched t))

(defun ide-mode-off ()
  "Disable the mode to use emacs as ide"
  (interactive)
  (delete-other-windows)
  (global-set-key [remap ide-mode-compile] 'compile)
  (global-set-key [remap ide-mode-eshell] 'eshell)
  (global-set-key [remap ide-mode-describe-key] 'describe-key)
  (kill-all-local-variables))

(provide 'ide-mode)
;;; ide-mode.el ends here
