# Getting Started

## Required Tools

The following are required tools for building and running Portage

### Go

The Portage CD is written in [Go](https://go.dev).

To install on a Mac, install using Homebrew:

```
brew install go
```

Optional: if you would like Go built tools to be available locally on the command line, add the following to your `~/.zshrc` or `~/.zprofile` file:

```
# Go
export GOPATH=$HOME/go
export PATH=$PATH:$GOPATH/bin
```

#### Recommended Resources

If you are new to Go, or would like a refresher, here are some recommended resources:

- [Go Documentation](https://go.dev/doc/effective_go)
- [101 Go Mistakes and How to Avoid Them](https://www.manning.com/books/100-go-mistakes-and-how-to-avoid-them) - A free an online summarized version can be found [here](https://github.com/teivah/100-go-mistakes)

## Optional Tools

The following are optional tools that may be installed to enhance the developer experience.

### mdbook

[mdbook](https://github.com/rust-lang/mdBook) is written in Rust and requires Rust to be installed as a pre-requisite.

To install Rust on a Mac or other Unix-like OS:

```
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh
```

If you've installed rustup in the past, you can update your installation by running:

```
rustup update
```

Once you have installed Rust, the following command can be used to build and install mdbook:

```
cargo install mdbook
```

Once mdbook is installed, you can serve it by going to the directory containing the mdbook markdown files and running:

```
mdbook serve
```

### just

[just](https://github.com/casey/just) is "just" a command runner. It is a handy way to save and run project-specific commands.

To install just on a Mac:

You can use the following command on Linux, MacOS, or Windows to download the latest release, just replace `<destination directory>` with the directory where you'd like to put just:

```
curl --proto '=https' --tlsv1.2 -sSf https://just.systems/install.sh | bash -s -- --to <destination directory>
```

For example, to install `just` to `~/bin`:

```
# create ~/bin
mkdir -p ~/bin

# download and extract just to ~/bin/just
curl --proto '=https' --tlsv1.2 -sSf https://just.systems/install.sh | bash -s -- --to ~/bin

# add `~/bin` to the paths that your shell searches for executables
# this line should be added to your shell's initialization file,
# e.g. `~/.bashrc` or `~/.zshrc`
export PATH="$PATH:$HOME/bin"

# just should now be executable
just --help
```
