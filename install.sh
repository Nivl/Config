#!/bin/bash

if [ -z "$PERSONAL_COMPUTER" ]; then
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
          printf "Invalid value\n"
      ;;
    esac
  done
fi

SKIP_EXISTING_CONFIG_FILES=false
while true; do
  printf "Do you want to skip copying existing config files (y/n)? "
  read -r answer

  case ${answer:0:1} in
    "y"|"Y" )
        SKIP_EXISTING_CONFIG_FILES=true
        break
    ;;
    "n"|"N" )
        break
    ;;
    * )
        printf "Invalid value\n"
    ;;
  esac
done

# install brew if not already installed
if [ -z "$HOMEBREW_PREFIX" ]; then
  /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"

  (echo; echo 'eval "$(/opt/homebrew/bin/brew shellenv)"') >> "$HOME/.zprofile"
  eval "$(/opt/homebrew/bin/brew shellenv)"
fi

# install/Update all softwares
brew install gnupg diff-so-fancy emacs pinentry-mac jq less grep zsh-syntax-highlighting shellcheck lsd gh
# fonts
brew install font-fira-code-nerd-font
# Install opinionated tools
brew install go golangci-lint go-task/tap/go-task nvm pnpm
# Install common apps
brew install --cask zoom brave-browser warp homebrew/cask/docker raycast keka slack
# install betas
brew install --cask visual-studio-code@insiders

if [ "$PERSONAL_COMPUTER" = true ]; then
  brew install --cask proton-drive proton-pass protonvpn daisydisk lulu ente yaak discord enpass
fi

# create default SSH key
if [ ! -e "$HOME/.ssh/default.pub" ]; then
  ssh-keygen -o -a 100 -t ed25519 -f "$HOME/.ssh/default"
fi
if [ ! -e "$HOME/.ssh/config" ]; then
  echo "IdentityFile $HOME/.ssh/default" > "$HOME/.ssh/config"
fi

CONFIG_DIR="$HOME/.melvin/config"

# Clone/update the repository
if [ ! -d "$CONFIG_DIR" ]; then
  mkdir -p "$CONFIG_DIR"
  cd "$CONFIG_DIR" || exit 1
  git clone git@github.com:Nivl/Config.git .
else
  cd "$CONFIG_DIR" || exit 1
  if [ -z "$(git status --porcelain)" ]; then 
   git pull
  else 
    printf "Your config repository has uncommited changes, please commit or stash them before we can update\n"
  fi
fi

# TODO(melvin):
# Enable auto update.

# Copy the config files over
FILES=(
  ".oh-my-zsh"
  ".emacs.d"
  ".bin-remote"
  ".golangci.yml"
)
for FILE_NAME in "${FILES[@]}"; do
  SOURCE="$CONFIG_DIR/$FILE_NAME"
  TARGET="$HOME/$FILE_NAME"

  if [ "$SKIP_EXISTING_CONFIG_FILES" = false ]; then
    if [ -e "$TARGET" ]; then
      while true; do
        printf "%s already exists\n" "$TARGET"
        printf "\t1) Backup and Overwrite\n"
        printf "\t2) Overwrite\n"
        printf "\t3) Skip\n"
        read -r answer

        case ${answer:0:1} in
          "1" )
              if [ -e "$TARGET.bpk" ]; then
                rm -rf "$TARGET.bpk"
              fi
              mv "$TARGET" "$TARGET.bpk"
              ln -s "$SOURCE" "$TARGET"
              break
          ;;
          "2" )
              rm -rf "$TARGET"
              ln -s "$SOURCE" "$TARGET"
              break
          ;;
          "3" )
              printf "Skipping\n"
              break
          ;;
          * )
              printf "Invalid value\n"
          ;;
        esac
      done
    else 
      ln -s "$SOURCE" "$TARGET"
    fi
  fi
done

mkdir -p "$HOME/.emacs-saves"

# if we don't have a base .zshrc, we create one with the default config
ZSHRC="$HOME/.zshrc"
if [ ! -e "$ZSHRC" ]; then
  {
    printf "source \"\$HOME/.melvin/config/base.zshrc\""
    printf "\n"
    printf "\nexport GIT_HOST=\"git@github.com\""
    printf "\nexport GIT_CLONE_USER_NAME=\"Nivl\""
    printf "\nexport PERSONAL_COMPUTER=\"%s\"" "$PERSONAL_COMPUTER"
  } > "$ZSHRC"
fi

# if we don't have a base .gitconfig, we create one with the default config
GITCFG="$HOME/.gitconfig"
if [ ! -e "$GITCFG" ]; then
  {
    printf "[include]\n\tpath = \"%s/.melvin/config/.gitconfig\"" "$HOME"

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

# Setup default gpg config
# https://dev.to/wes/how2-using-gpg-on-macos-without-gpgtools-428f
if [ ! -e "$HOME/.gnupg/gpg-agent.conf" ]; then
  mkdir -p "$HOME/.gnupg"
  P="$HOMEBREW_PREFIX/bin/pinentry-mac"
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
