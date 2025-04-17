# Get Shit Done (GSD)

A simple CLI tool to help you stay focused by blocking distracting websites using your `/etc/hosts` file.

Inspired by [viccherubini/get-shit-done](https://github.com/viccherubini/get-shit-done), this Go-based version supports easy configuration, automation, and packaging.

## Features

- 🔒 Block sites via `work` mode
- 🎉 Unblock with `play` mode
- ➕ Manage block list with `add` and `remove`
- 📋 View current block list with `list`
- ✅ Check current mode with `status`
- 💻 Includes `.deb` packaging, man page, and bash completion

## Usage

```bash
sudo getshitdone work           # Block domains
sudo getshitdone play           # Unblock domains
sudo getshitdone add reddit.com
sudo getshitdone remove twitter.com
sudo getshitdone list
sudo getshitdone status
```

## Config

Blocked domains are stored in:

```
~/.config/get-shit-done.ini
```

Override with:

```
--config=/path/to/custom.ini
```

## Installation

Build and install from source:

```bash
make
sudo make install
```

## License

MIT


