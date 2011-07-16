;; Charger le mode C/C++
(require 'cc-mode)


;; Définition d'un style (i.e. une mise en page) conforme à mes petites
;; habitudes. La signification des différents paramètres est expliquée
;; dans le manuel du mode CC, notamment pour les indentations la page :
;; http://www.delorie.com/gnu/docs/emacs/cc-mode_32.html
(defconst my-c-style
  '(;; L'appui sur la touche « tabulation » ne doit pas insérer une
    ;; tabulation mais indenter la ligne courante en fonction du
    ;; contexte et des règles définies dans le style.
    (c-tab-always-indent . t)
    ;; Formatage à 78 colonnes
    (fill-column . 78)
    ;; L'indentation se fait avec un pas de 2 caractères
    (c-basic-offset . 2)
    ;; Les commentaires qui occupent seuls une ligne sont alignés avec
    ;; le code
    (c-comment-only-line-offset . 0)
    ;; Les commentaires multi-ligne commencent par une simple ligne '/*'
    (c-hanging-comment-starter-p . t)
    ;; et se terminent par une simple ligne '*/'
    (c-hanging-comment-ender-p . t)
    ;; Cas où une accolade est « électrique » (i.e. provoque une mise
    ;; en page automatique)
    (c-hanging-braces-alist .
      ((substatement-open after)
       (brace-list-open)
       (brace-entry-open)
       (block-close . c-snug-do-while)
       (extern-lang-open after)
       (inexpr-class-open after)
       (inexpr-class-close before)))
    ;; Cas où le caractère « : » est « électrique » (i.e. provoque une
    ;; mise en page automatique)
    (c-hanging-colons-alist .
      ((member-init-intro before)
       (inher-intro)
       (case-label after)
       (label after)
       (access-label after)))
    ;; Nettoyage automatique de certaines mises en page
    (c-cleanup-list .
      (scope-operator
       empty-defun-braces
       defun-close-semi))
    (c-offsets-alist .
      (;; Première ligne d'une construction de premier niveau (par
       ;; exemple une déclaration de fonction)
       (topmost-intro . 0)
       ;; Lignes suivantes d'une construction de premier niveau
       (topmost-intro-cont . 0)
       ;; Première ligne d'une liste d'argument
       (arglist-intro . +)
       ;; Argument lorsque la ligne ouvrant la liste ne contient pas
       ;; d'argument.
       (arglist-cont . 0)
       ;; Argument lorsque la ligne ouvrant la liste en contient au moins un.
       (arglist-cont-nonempty . c-lineup-arglist)
       ;; Parenthèse fermant une liste d'arguments mais non précédée d'un
       ;; argument sur la même ligne.
       (arglist-close . c-lineup-close-paren)
       ;; Première ligne d'une instruction quelconque
       (statement . 0)
       ;; Lignes suivantes de l'instruction quelconque
       (statement-cont . +)
       ;; Première ligne d'un bloc
       (statement-block-intro . +)
       ;; Première ligne d'un bloc case
       (statement-case-intro . +)
       ;; Première ligne d'un bloc case commençant par une accolade
       (statement-case-open . 0)
       ;; Instruction suivant une instruction de test ou de contrôle de boucle
       (substatement . +)
       ;; Accolade suivant une instruction de test ou de contrôle de boucle
       (substatement-open . 0)
       ;; Accolade ouvrante d'une énumération ou d'un tableau statique
       (brace-list-open . 0)
       ;; Accolade fermante d'une énumération ou d'un tableau statique
       (brace-list-close . 0)
       ;; Première ligne d'une énumération ou d'un tableau statique
       (brace-list-intro . +)
       ;; Lignes suivantes d'une énumération ou d'un tableau statique
       (brace-list-entry . 0)
       ;; Lignes suivantes d'une énumération ou d'un tableau statique
       ;; commençant par une accolade ouvrante
       (brace-entry-open . 0)
       ;; Label d'un switch
       (case-label . +)
       ;; Label d'une classe (public, protected, private) en retrait d'un pas
       ;; par rapport à l'indentation normale au sein d'une classe (cf.
       ;; déclaration « inclass » plus bas).
       (access-label . -)
       ;; Autres labels
       (label . 0)
       ;; Ouverture de bloc
       (block-open . 0)
       ;; Fermeture de bloc
       (block-close . 0)
       ;; A l'intérieur d'une chaîne multi-ligne
       (string . c-lineup-dont-change)
       ;; Première ligne d'un commentaire
       (comment-intro . c-lineup-comment)
       ;; A l'intérieur d'un commentaire C multi-ligne
       (c . c-lineup-C-comments)
       ;; Accolade ouvrant une fonction
       (defun-open . 0)
       ;; Accolade fermant une fonction
       (defun-close . 0)
       ;; Code suivant l'accolade ouvrante d'une fonction
       (defun-block-intro . +)
       ;; Clause else d'une expression conditionnelle
       (else-clause . 0)
       ;; Clause catch d'une instruction try
       (catch-clause . 0)
       ;; Accolade ouvrant une déclaration de classe
       (class-open . 0)
       ;; Accolade fermant la déclaration de classe
       (class-close . 0)
       ;; Accolade ouvrante d'une méthode définie dans la classe elle-même
       ;; (inline)
       (inline-open . 0)
       ;; Accolade fermante de la méthode inline
       (inline-close . 0)
       ;; Alignement des opérateurs de flux (<< et >>) sur les opérateurs de
       ;; flux de la ligne précédente
       (stream-op . c-lineup-streamop)
       ;; Ligne incluse dans une déclaration de classe (double indentation car
       ;; les labels d'accès public, protected et private sont déjà indentés)
       (inclass . ++)
       ;; Accolade ouvrant un bloc en langage externe (extern "C" {})
       (extern-lang-open . 0)
       ;; Accolade fermant un bloc en langage externe
       (extern-lang-close . 0)
       ;; Indentation dans un bloc de langage externe
       (inextern-lang . +)
       ;; Accolade ouvrant un bloc d'espace de nom
       (namespace-open . 0)
       ;; Accolade fermant un bloc d'espace de nom
       (namespace-close . 0)
       ;; Indentation dans un bloc d'espace de nom
       (innamespace . +)
       ;; Première ligne d'héritage
       (inher-intro . +)
       ;; Lignes suivantes d'héritage
       (inher-cont . c-lineup-multi-inher)
       ;; Première ligne de la liste d'initialisation
       (member-init-intro . +)
       ;; Lignes suivantes de la liste d'initialisation
       (member-init-cont . c-lineup-multi-inher)
       ;; Lignes entre la déclaration de fonction et l'accolade ouvrante. En
       ;; C, il n'y a rien mais en C++, il y a les listes d'initialisation
       (func-decl-cont . +)
       ;; Première ligne d'une macro (avec un décalage négatif excessif afin
       ;; d'être certain qu'elle reste collée à gauche en toute circonstance
       (cpp-macro . -1000)
       ;; Lignes suivantes d'une macro
       (cpp-macro-cont . c-lineup-dont-change)
       ;; Fonction amie
       (friend . 0)
       ;; while qui termine une instruction do { ... } while (...);
       (do-while-closure . 0)
       ;; Bloc d'instruction à l'intérieur d'une expression
       (inexpr-statement . 0)
       ;; Définition de classe à l'intérieur d'une expression (cela n'a de
       ;; sens qu'en Java mais autant définir ce contexte au cas où...)
       (inexpr-class . +)
       ;; Lignes autres que la première d'un modèle de fonction ou de classe
       (template-args-cont . +)
       ;; Arguments d'une fonction à la sauce K&R
       (knr-argdecl-intro . +)
       (knr-argdecl . 0)))
    (c-echo-syntactic-information-p . t)
  )
  "My C Programming Style"
)


;; Faire du style défini ci-dessus le style C/C++ par défaut
(defun my-c-mode-common-hook ()
  (c-add-style "PERSONAL" my-c-style t)
)
(add-hook 'c-mode-hook 'my-c-mode-common-hook)
(add-hook 'c++-mode-hook 'my-c-mode-common-hook)
(add-hook 'php-mode-user-hook 'my-c-mode-common-hook)
