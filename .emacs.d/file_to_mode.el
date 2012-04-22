
; Edit header.el when adding new mode.
(setq auto-mode-alist
      (append
       '(("\\.sh\\'" . sh-mode)
	 ("bash" . sh-mode)
	 ("profile" . sh-mode)
	 ("Makefile\\'" . makefile-mode)
	 ("makefile\\'" . makefile-mode)
	 ("\\.mk\\'" . makefile-mode)
	 ("\\.c\\'"  . c-mode)
	 ("\\.h\\'"  . c-mode)
	 ("\\.cc\\'" . c++-mode)
	 ("\\.hh\\'" . c++-mode)
	 ("\\.cpp\\'"  . c++-mode)
	 ("\\.hpp\\'"  . c++-mode)
	 ("\\.pgc\\'"  . c++-mode) ; Fichiers « Embedded PostgreSQL in C »
	 ("\\.p[lm]\\'" . cperl-mode)
	 ("\\.el\\'" . emacs-lisp-mode)
	 ("\\.emacs\\'" . emacs-lisp-mode)
	 ("\\.l\\'" . lisp-mode)
	 ("\\.lisp\\'" . lisp-mode)
	 ("CMakeLists\\.txt\\'" . cmake-mode)
	 ("\\.cmake\\'" . cmake-mode)
	 ("\\.sgml\\'" . sgml-mode)
	 ("\\.xml\\'" . sgml-mode)
	 ("\\.xsl\\'" . sgml-mode)
	 ("\\.svg\\'" . sgml-mode)
	 ("\\.py\\'" . python-mode)
	 ("\\.js\\'" . javascript-mode)
	 ("\\.css\\'" . css-mode)
	 ("\\.less\\'" . css-mode)
	 ("\\.tpl\\'" . html-helper-mode)
	 ("\\.inc\\'" . php-mode)
	 ("\\.awk\\'" . awk-mode)
	 ("\\.tex\\'" . latex-mode)
	 ("\\.yml\\'" . yaml-mode)
	 ("\\.cfg\\'" . c-mode)
	 ("\\.yml\\'" . yaml-mode)
	 ("\\.yaml\\'" . yaml-mode)
	 ("\\.pov\\'" . pov-mode)
	 ("\\.java\\'" . java-mode)
	 ("\\.md\\'" . markdown-mode)
	 ("\\.ml[iylp]?\\'" . tuareg-mode)
	 ("/PKGBUILD$" . pkgbuild-mode)
	 ;("\\.po\\'" . po-mode) gettext-el
	 )
       auto-mode-alist))



(setq nivl-mumamo-regex ".+\\.\\(php\\|[sxd]?html\\)$")
(setq nivl-mumamo-mode-alist
      '(
	("\\.[sxd]?html?\\'" . django-html-mumamo-mode)
	("\\.php\\'" . html-mumamo-mode)
	))



(mapc (lambda (list)
	(mapc (lambda (pair)
		(if (or (eq (cdr pair) 'html-mode)
			(eq (cdr pair) 'php-mode))
		    (setcdr pair (lambda ()
				   (require 'nxhtml-mode "~/.emacs.d/autoloadable/nxhtml/autostart.el")
				   (setq mumamo-background-colors nil)
				   (setq auto-mode-alist
					 (append
					  nivl-mumamo-mode-alist
					  auto-mode-alist))
				   (when (file-exists-p (buffer-file-name))
				     (revert-buffer t t))))))
	      list))
      (list auto-mode-alist magic-mode-alist))
