#!/bin/bash

USE_APP_STORE=false
while true; do
  echo -n "Use the Mac AppStore to install apps when possible (Y/n)? "
  read answer

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
  echo "source \"\$HOME/My Drive/unix_conf/base.zshrc\"" > "$ZSHRC"
fi

# if we don't have a base .gitconfig, we create one with the default config
GITCFG="$HOME/.gitconfig"
if [ ! -e "$GITCFG" ]; then
  echo "[include]\n\tpath = \"$HOME/My Drive/unix_conf/.gitconfig\"" > "$GITCFG"
fi

# install all softwares
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
brew tap homebrew/cask
brew tap homebrew/cask-fonts
brew install gnupg diff-so-fancy emacs pinentry-mac jq zsh brew-cask-completion less font-fira-code zsh-syntax-highlighting shellcheck
# Install opinionated tools
brew install go golangci-lint go-task/tap/go-task
# Install common apps
brew install --cask zoom brave-browser loom visual-studio-code app-cleaner homebrew/cask-drivers/1kc-razer homebrew/cask-drivers/logitech-options homebrew/cask-drivers/logitech-firmwareupdatetool
# install betas
brew install --cask homebrew/cask-versions/iterm2-beta

# Some apps are still not available on ARM
IS_ARM=true
if [ ! -e "/opt/homebrew/" ]; then
  IS_ARM=false
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

# switch shell to zsh
chsh -s "$(which zsh)"

echo "Things left to do:"
echo "\t1. Switch to ZSH and run 'compaudit | xargs chmod g-w,o-w'"
echo "\t2. Don't forget to upload $HOME/.ssh/default to github: 'pbcopy < $HOME/.ssh/default.pub'"
echo "\t3. Import PGP Key from Enpass with 'gpg --import private.key"
if [ "$USE_APP_STORE" = false ]; then
  echo "\t4. Install EasyRes: http://easyresapp.com"
fi
