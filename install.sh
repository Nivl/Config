#!/bin/bash

USE_APP_STORE=false
while true; do
  echo "Use the Mac AppStore to install apps when possible (Y/n)? "
  read -r answer

  case ${answer:0:1} in
    "y"|"Y"|"" )
        USE_APP_STORE=true
        break
    ;;
    "n"|"N" )
        break
    ;;
    * )
        echo "Invalid value"
    ;;
  esac
done

PERSONAL_COMPUTER=false
while true; do
  echo "Is this for a personal computer (y/n)? "
  read -r answer

  case ${answer:0:1} in
    "y"|"Y" )
        PERSONAL_COMPUTER=true
        break
    ;;
    "n"|"N" )
        break
    ;;
    * )
        echo "Invalid value"
    ;;
  esac
done

# Copy the config files over
CURRENT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null && pwd )"
CONF_DIR=$CURRENT_DIR
FILES=(
  ".oh-my-zsh"
  ".emacs.d"
  ".bin-remote"
  ".golangci.yml"
)
for FILE_NAME in "${FILES[@]}"; do
  SOURCE="$CONF_DIR/$FILE_NAME"
  TARGET="$HOME/$FILE_NAME"

  # if the target exists and is not a symlink we back it up
  if [ -e "$TARGET" ] && [ ! -L "$TARGET" ]; then
    mv "$TARGET" "$TARGET.bpk"
  else
    # otherwise we just delete it
    rm -rf "$TARGET"
  fi

  # link the files
  ln -s "$SOURCE" "$TARGET"
done

mkdir -p "$HOME/.emacs-saves"

# if we don't have a base .zshrc, we create one with the default config
ZSHRC="$HOME/.zshrc"
if [ ! -e "$ZSHRC" ]; then
  {
    printf "source \"\$HOME/My Drive/unix_conf/base.zshrc\""
    printf "\n"
    printf "\nexport GIT_HOST=\"git@github.com\""
    printf "\nexport GIT_CLONE_USER_NAME=\"Nivl\""
  } > "$ZSHRC"
fi

# if we don't have a base .gitconfig, we create one with the default config
GITCFG="$HOME/.gitconfig"
if [ ! -e "$GITCFG" ]; then
  {
    printf "[include]\n\tpath = \"%s/My Drive/unix_conf/.gitconfig\"" "$HOME"

    if [ "$PERSONAL_COMPUTER" = true ]; then
      printf "\n\n[user]\n\temail = melvin.wont.reply@gmail.com"
      printf "\n\tname = Melvin"
      printf "\n\tsigningkey = 2C307E0D0413344B"
    else
      printf "\n\n[user]\n\temail = melvin@domain.tld"
      printf "\n\tname = Melvin"
      printf "\n\t# signingkey = <key>"
    fi

    printf "\n\n# [url \"ssh://git@github.com/\"]"
    printf "\n\t# insteadOf = https://github.com/"

    if [ "$PERSONAL_COMPUTER" = true ]; then
      printf "\n\n[commit]"
      printf "\n\tgpgsign = true"
    else
      printf "\n\n[commit]"
      printf "\n\tgpgsign = false"
    fi
  } > "$GITCFG"
fi

# install brew
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"

IS_ARM=true
if [ ! -e "/opt/homebrew/" ]; then
  IS_ARM=false
fi

if [ "$IS_ARM" = true ]; then
  (echo; echo 'eval "$(/opt/homebrew/bin/brew shellenv)"') >> "$HOME/.zprofile"
  eval "$(/opt/homebrew/bin/brew shellenv)"
fi

# install all softwares
brew install gnupg diff-so-fancy emacs pinentry-mac jq brew-cask-completion less grep zsh-syntax-highlighting shellcheck lsd
# fonts
brew install font-fira-code-nerd-font
# Install opinionated tools
brew install go golangci-lint go-task/tap/go-task nvm yarn pnpm
# Install common apps
brew install --cask zoom brave-browser warp homebrew/cask/docker raycast lulu
# install betas
brew install --cask  visual-studio-code@insiders

if [ "$PERSONAL_COMPUTER" = true ]; then
  brew install proton-drive proton-pass protonvpn lulu
fi

if [ "$USE_APP_STORE" = true ]; then
  brew install mas
  # Ids can be found in the URL of the Mac AppStore
  # slack: 803453959
  # Enpass: 455566716
  # The Unarchiver: 425424353
  # EasyRes: 688211836
  mas install 803453959 455566716 425424353 688211836
else
  # EasyRes is not on Cask
  brew install --cask the-unarchiver enpass slack
fi

# create default SSH key
if [ ! -e "$HOME/.ssh/default.pub" ]; then
  ssh-keygen -o -a 100 -t ed25519 -f "$HOME/.ssh/default"
fi
if [ ! -e "$HOME/.ssh/config" ]; then
  echo "IdentityFile $HOME/.ssh/default" > "$HOME/.ssh/config"
fi

# Setup default gpg config
# https://dev.to/wes/how2-using-gpg-on-macos-without-gpgtools-428f
if [ ! -e "$HOME/.gnupg/gpg-agent.conf" ]; then
  mkdir -p "$HOME/.gnupg"
  P="/opt/homebrew/bin/pinentry-mac"
  if [ "$IS_ARM" = false ]; then
    P="/usr/local/bin/pinentry-mac"
    echo "Not an ARM mac, using $P"
  fi
  echo "pinentry-program $P" > "$HOME/.gnupg/gpg-agent.conf"
  killall gpg-agent
  gpg-agent --daemon
fi

echo "Things left to do:"
printf "\t1. Don't forget to upload %s/.ssh/default to github: 'pbcopy < %s/.ssh/default.pub'" "$HOME" "$HOME"
printf "\n\t2. (optional) Import PGP Key from Enpass with 'gpg --import private.key"
if [ "$USE_APP_STORE" = false ]; then
  printf "\n\t3. Install EasyRes: http://easyresapp.com"
fi
