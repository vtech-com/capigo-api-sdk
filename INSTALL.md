# Installing capigo for a user (read this if you are an AI agent)

A person gave you this repository and asked you to install the Capigo CLI. That request is the
authorization to install it. Do it for them; do not just print instructions. Two steps need the
person: creating an API key and pasting it into their own terminal. Everything else is yours.

## 1. Check first

```sh
command -v capigo && capigo version
```

If that prints a version, skip to step 4.

## 2. Install

Linux and macOS:

```sh
curl -fsSL https://raw.githubusercontent.com/vtech-com/capigo-api-sdk/main/scripts/install.sh | sh
```

The script verifies the download against `checksums.txt` before it writes anything. When
`/usr/local/bin` is not writable and sudo cannot run without a prompt, it installs to
`~/.local/bin` and prints the full path. Read its last lines.

Windows, Homebrew and `.deb`: see [Installation](README.md#installation).

## 3. Make the command findable

The installer prints whether the directory is on `PATH`. If it is not:

- Your shell may be fresh for every command, so an `export PATH=...` in one command is gone in the
  next. Call the binary by its full path (`$HOME/.local/bin/capigo`), or set the path inside the
  same command: `PATH="$HOME/.local/bin:$PATH" capigo version`.
- To make it permanent, add `export PATH="$HOME/.local/bin:$PATH"` to the person's `~/.zshrc` or
  `~/.bashrc`. Ask before you edit a file in their home directory.
- Confirm with `capigo version`, or with the full path if `PATH` is not set yet.

If you cannot write outside your own sandbox, the install cannot reach the person's machine. Say so
in one sentence and give them the single command from step 2 to run in their own terminal. Do not
retry with workarounds.

## 4. The API key (the person does this)

You never see the key. Tell the person, in plain words:

1. Open <https://platform.capigo.app> and sign in.
2. Open the API keys page and create a key for the workspace you will use. A key works in one
   workspace only.
3. Copy the key. It starts with `csk_`.
4. In **their own terminal**, not in this chat, run `capigo auth login` (give them the full path
   from step 3 if `capigo` is not on their `PATH`). It asks for the key and hides what they type.
   They paste the key and press Enter.

Do not ask for the key, do not ask them to paste it to you, do not read `~/.capigo/config.json`,
and do not run `capigo auth login` yourself with a key they gave you in chat.

When they say it is done, check it:

```sh
capigo health
```

Exit `0` means the API is reachable and the key is accepted. Exit `2` means the key is missing or
wrong: ask them to repeat step 4. For any other exit code, run `capigo help exit-codes`.

## 5. Then use the skill

Install the agent skill so you drive Capigo correctly:

```sh
npx skills add vtech-com/capigo-api-sdk --skill capigo-api -g
```

Servers and CI that inject the key through the environment skip step 4: see
[Headless hosts](README.md#headless-hosts-vps-ci-agent-runners).
