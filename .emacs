;; MEMO:
;; Pour trouver le nom d'une commande a partir d'un shortcut : C-h c

(setq enable_epitech t)

(load-file "~/.emacs.d/my-autoload.el")
(my-autoload "~/.emacs.d/autoloadable" 1)
(load-file "~/.emacs.d/compile.el")
(compile-uncompiled-files)

(require 'package)
(add-to-list 'package-archives
             '("elpa" . "http://tromey.com/elpa/"))
(add-to-list 'package-archives
	     '("marmalade" . "http://marmalade-repo.org/packages/"))
(package-initialize)


(require 'redo+)

(autoload 'php-mode "php-mode" "Major mode for php" t)
(autoload 'pkgbuild-mode "pkgbuild-mode" "Major mode for editing PKGBUILD." t)

(autoload 'ide-mode "ide-mode" "Mode to use emacs as ide" t)

(require 'auto-complete-config)
(add-to-list 'ac-dictionary-directories "~/.emacs.d/autoloadable/auto-complete/dict")
(ac-config-default)


(require 'yasnippet)
(yas/initialize)
(setq yas/snippet-dirs '("~/.emacs.d/snippets"
			 "~/.emacs.d/custom_snippets"
			 ))
(mapc 'yas/load-directory yas/snippet-dirs)
(yas/global-mode 1)
(require 'dropdown-list)
(setq yas/prompt-functions '(yas/dropdown-prompt
			     yas/ido-prompt
			     yas/completing-prompt))

(load-file "~/.emacs.d/main_options.elc")
(load-file "~/.emacs.d/shortcuts.elc")
(load-file "~/.emacs.d/insert.elc")
(load-file "~/.emacs.d/file_to_mode.elc")
(load-file "~/.emacs.d/hooks.elc")
(load-file "~/.emacs.d/misc_functions.elc")
(load-file "~/.emacs.d/main_options.elc")
(load-file "~/.emacs.d/templates.elc")
(load-file "~/.emacs.d/headers.elc")

(when enable_epitech
  (require 'epitech-mode)
  )
