---
layout: default
title: "Install Zephyr on macOS"
permalink: /install/
---

<nav>
  <a href="{{ '/' | relative_url }}">Zephyr</a>
  <a href="{{ '/' | relative_url }}#features">Features</a>
  <a href="{{ '/' | relative_url }}#download">Downloads</a>
  <a href="{{ site.repo }}">GitHub</a>
</nav>

<div class="container install-content" markdown="1">

# Install Zephyr on macOS

Zephyr requires macOS 12 or later.

The supported macOS installation and upgrade path is one Terminal command:

<div class="command-copy">
  <pre><code>curl -fsSL https://raw.githubusercontent.com/kkollsga/zephyr/main/install.sh | bash</code></pre>
  <button type="button" class="copy-command" onclick="copyInstallCommand(this)" aria-label="Copy macOS install command" aria-live="polite">Copy</button>
</div>

The installer selects the latest release, requires and verifies its release
SHA-256 checksum, verifies the app bundle's code signature, and installs the app at
`/Applications/Zephyr.app`. It also clears download quarantine, registers the
app with Launch Services, and creates `/usr/local/bin/zephyr`.

Current releases are ad-hoc signed. Signature verification detects bundle
corruption but does not authenticate an Apple-verified publisher or provide
notarization.

Open Zephyr or confirm the installed version:

```bash
open /Applications/Zephyr.app
zephyr --version
```

## Upgrade

Run the same command. The installer closes a running copy of Zephyr, stages the
new bundle, and restores the previous app automatically if replacement fails.

Use the command above again whenever you want to update.

Your settings, themes, and recent projects remain in `~/.config/zephyr` and are
not replaced during an upgrade.

## Install a specific release

```bash
curl -fsSL https://raw.githubusercontent.com/kkollsga/zephyr/main/install.sh | \
  bash -s -- --version v1.2.3
```

## Install without administrator access

Install the app and command inside your home directory:

```bash
curl -fsSL https://raw.githubusercontent.com/kkollsga/zephyr/main/install.sh | \
  bash -s -- --install-dir "$HOME/Applications" --bin-dir "$HOME/.local/bin"
```

Add `$HOME/.local/bin` to your shell `PATH` if it is not already present.

## Review the installer before running it

```bash
curl -fsSLO https://raw.githubusercontent.com/kkollsga/zephyr/main/install.sh
less install.sh
bash install.sh
rm install.sh
```

## Manual DMG installation

Download the DMG from the [latest GitHub release]({{ site.release_url }}), open
it, and drag **Zephyr.app** to Applications. The Terminal command at the top of
this guide is the supported path because it handles system setup and future
upgrades automatically.

## Command-line options

```text
--version TAG       Install a specific release
--install-dir DIR   Choose the application directory
--bin-dir DIR       Choose where the zephyr command is linked
--no-cli            Do not install a terminal command
--open              Open Zephyr after installation
```

## Uninstall

```bash
sudo rm -rf /Applications/Zephyr.app
sudo rm -f /usr/local/bin/zephyr
```

To also remove personal settings and themes:

```bash
rm -rf ~/.config/zephyr
```

</div>
