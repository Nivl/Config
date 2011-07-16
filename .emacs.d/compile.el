;;
;; compile.el
;;
;; Made by Melvin Laplanche <melvin.laplanche+dev@gmail.com>
;;
;; Started on  Sun Jun 19 16:45:25 2011 Melvin Laplanche
;; Last update Tue Jun 21 19:17:33 2011 Melvin Laplanche
;;

(defun compile-uncompiled-files ()
  ;(compile-uncompiled-files-process "~/")
  (compile-uncompiled-files-process "~/.emacs.d/" t)
  (compile-uncompiled-files-process "~/.emacs.d/autoloadable/" t)
  )

; remove: Remove an .elc file if his .el file is not found in the
;         same directory.
; recur: Browse directories recursively
(defun compile-uncompiled-files-process (dir &optional remove recur)
  "Compiles all uncompiled and not up-to-date .el files from dir"
  (setq dir (file-name-as-directory dir))
  (setq files (directory-files dir))
  (dolist (file files)
    (if (and
	 (and (> (length file) 3)
	      (equal ".el" (substring file -3 nil)))
	 (and (file-writable-p (concat dir file "c"))
	      (or (not (file-exists-p (concat dir file "c")))
		  (file-newer-than-file-p (concat dir file)
					  (concat dir file "c")))))
	(byte-compile-file (concat dir file))
      (progn
	(if (and (equal remove t)
		 (> (length file) 4)
		 (equal ".elc" (substring file -4 nil)))
	    (when (not (file-exists-p (concat dir (substring file 0 -1))))
	      (message "removed %s" (concat dir file))
	      (delete-file (concat dir file)))
	  (when (and
		 (equal t recur)
		 (not (or (equal file ".")
			  (equal file "..")))
		 (equal t (file-directory-p (concat dir file))))
	    (compile-uncompiled-files-process (concat dir file) t)
	    ))))))

