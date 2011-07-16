
(load-file "~/.emacs.d/compile.el")
(compile-uncompiled-files)

;; MEMO:
;; Pour trouver le nom d'une commande a partir d'un shortcut : C-h c

;; Load CEDET (Collection of Emacs Development Environment Tools)
; (load-file "/usr/share/emacs/site-lisp/cedet/common/cedet.el")

(add-to-list 'load-path "~/.emacs.d/autoloadable")
(add-to-list 'load-path "~/.emacs.d/autoloadable/html5-el")
;(add-to-list 'load-path "~/.emacs.d/autoloadable/nxhtml")
(require 'redo+)

(autoload 'php-mode "php-mode" "Major mode for PHP" t)
;(autoload 'javascript-mode "javascript-mode" "Major mode for Javascript" t)
(autoload 'pov-mode "pov-mode" "Major mode for POV-Ray" t)
(autoload 'yaml-mode "yaml-mode" "Major mode for Yaml" t)
(autoload 'cmake-mode "cmake-mode" "Major mode for CMake" t)
(autoload 'python-mode "python-mode" "Major mode for python" t)
(autoload 'django-mode "django-mode" "Major mode for Django" t)
(autoload 'django-html-mode "django-html-mode" "Major mode for Django template" t)
(autoload 'pkgbuild-mode "pkgbuild-mode" "Major mode for editing PKGBUILD." t)

(autoload 'ide-mode "ide-mode" "Mode to use emacs as ide" t)

(require 'yasnippet)
(yas/initialize)
(setq yas/snippet-dirs "~/.emacs.d/snippets")
(yas/load-directory yas/snippet-dirs)
(require 'dropdown-list)
(setq yas/prompt-functions '(yas/dropdown-prompt
			     yas/ido-prompt
			     yas/completing-prompt))

; Gère plein de mode (html, php, ...)
;(load "~/.emacs.d/autoloadable/nxhtml/autostart.el")
;(setq mumamo-background-colors nil)

; Android dev
(require 'android-mode)
(setq android-mode-sdk-dir "/opt/android-sdk")

(load-file "~/.emacs.d/main_options.elc")
(load-file "~/.emacs.d/shortcuts.elc")
(load-file "~/.emacs.d/insert.elc")
(load-file "~/.emacs.d/file_to_mode.elc")
(load-file "~/.emacs.d/misc_functions.elc")
(load-file "~/.emacs.d/main_options.elc")
(load-file "~/.emacs.d/templates.elc")
(load-file "~/.emacs.d/headers.elc")
(load-file "~/.emacs.d/hooks.elc")
;(load-file "~/.emacs.d/my_c_style.elc")

;(require 'epitech-mode)
