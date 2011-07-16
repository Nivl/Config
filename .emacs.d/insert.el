;;; insert.el ---

(defun insert-date ()
  "Insert the current date."
  (interactive)
  (insert (get-date)))

(defun insert-filename ()
  "Insert the filename."
  (interactive)
  (insert (get-filename)))

(defun insert-filename-no-ext ()
  "Insert the filename without extension."
  (interactive)
  (insert (get-filename-no-ext)))

;(defun insert-protect-header ()
;  "Inserts a define to protect the header file."
;  (interactive)
;  (insert (get-protect-header))
;  (previous-line)
;  (previous-line))
