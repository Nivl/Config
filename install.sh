#!/bin/bash

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
      printf "\n\n[user]\n\temail = noreply@melvin.la"
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


(echo; echo 'eval "$(/opt/homebrew/bin/brew shellenv)"') >> "$HOME/.zprofile"
eval "$(/opt/homebrew/bin/brew shellenv)"
 
# install all softwares
brew install gnupg diff-so-fancy emacs pinentry-mac jq less grep zsh-syntax-highlighting shellcheck lsd gh
# fonts
brew install font-fira-code-nerd-font
# Install opinionated tools
brew install go golangci-lint go-task/tap/go-task nvm pnpm
# Install common apps
brew install --cask zoom brave-browser warp homebrew/cask/docker raycast keka slack
# install betas
brew install --cask  visual-studio-code@insiders

if [ "$PERSONAL_COMPUTER" = true ]; then
  brew install proton-drive proton-pass protonvpn daisydisk lulu ente yaak discord enpass
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
  echo "pinentry-program $P" > "$HOME/.gnupg/gpg-agent.conf"
  killall gpg-agent
  gpg-agent --daemon
fi


SETUP_GITHUB=false
if [ "$(gh auth status -a --json hosts --jq '.hosts["github.com"][0].state')" != "success" ]; then
  while true; do
    echo "Setup Github (y/n)? "
    read -r answer

    case ${answer:0:1} in
      "y"|"Y" )
          SETUP_GITHUB=true
          gh auth login -w # will ask to upload the previously generated SSH key to Github, and set it as default for git operations
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
else 
  SETUP_GITHUB=true
fi

echo "Things left to do:"

if [ "$SETUP_GITHUB" = false ]; then
  printf "\t1* Upload %s/.ssh/default to your Cloud VCS: 'pbcopy < %s/.ssh/default.pub'" "$HOME" "$HOME"
fi

easyRes=$(mdfind "kMDItemKind == 'Application'" | grep -iF "EasyRes")
if [ $? -eq 1 ]; then
  printf "\n\t* (optional) Install EasyRes if needed: http://easyresapp.com"
fi

printf "\n\t* (optional) Import PGP Key from Enpass with 'gpg --import private.key"
