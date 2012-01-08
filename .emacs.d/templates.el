(defun replace-all (to-find replacement)
  (beginning-of-buffer)
  (replace-string to-find replacement)
  )

(defun template-insert ()
  (replace-all "INSERT-FILE-NAME-NO-EXT" (get-filename-no-ext))
  (replace-all "INSERT-FILE-NAME" (get-filename))
  (replace-all "INSERT-CURRENT-DATE" (get-date))
  (end-of-buffer)
  (previous-line)
  (previous-line)
  (insert "  ")
  )

(defun template-insert-h ()
  (template-insert)
  (replace-all "INSERT-HEADER-PROTECTION" (get-protect-header))
  (end-of-buffer)
  (previous-line)
  (previous-line)
  (previous-line)
  )


(when (not enable_epitech)
  (add-hook 'find-file-hooks 'auto-insert)
  (load-library "autoinsert")
  (setq auto-insert-directory "~/.emacs.d/templates/")
  (setq auto-insert-query nil)
  (setq auto-insert-alist
	(append '((php-mode . ["php_min.php" template-insert])
		  (nxml-mode . ["html.html" template-insert])
		  ("\\.h\\'" . ["c_min.h" template-insert-h])
		  ("\\.hpp\\'" . ["cpp_min.hpp" template-insert-h])
		  (c++-mode . ["cpp_min.c" template-insert])
		  (c-mode . ["c_min.c" template-insert]))
		auto-insert-alist))
  )
