# sermo

`sermo` is a terminal client for Sermo forums. It looks like a bulletin board system from the 1990s, with modern truecolor graphics. The default server is https://fcuba.net.

With `sermo`, you can read forums and threads, write replies, react to posts, and read your notifications and conversations. The client has only member functions. It has no staff, moderation or administration functions.

## Requirements

- A Sermo server.
- A terminal. Truecolor is better, but `sermo` also works with 256 colors, 16 colors and no color.
- For `sermo login`: a server that has device login. If the server does not have device login, make an API key in your account and put it in the `SERMO_API_KEY` environment variable.

You can read public forums as a guest. You do not need an account for that.

## Install

### Release archives

1. Go to the GitHub Releases page of `albertgmz/sermo-cli`.
2. Download the archive for your system. Archives are available for Windows, macOS and Linux, on amd64 and arm64. Windows archives are `.zip` files. The other archives are `.tar.gz` files.
3. Extract the archive.
4. Put the `sermo` binary in a directory that is in your `PATH`.

### Go

```sh
go install github.com/albertgmz/sermo-cli/cmd/sermo@latest
```

### Update

```sh
sermo update
```

This command downloads the latest release and replaces the binary. It does a check of the checksum before it replaces the binary. A development build cannot update itself.

## Usage

| Command | Function |
| --- | --- |
| `sermo` | Starts the full screen client. |
| `sermo login` | Signs you in with your account in the browser. |
| `sermo logout` | Ends the session of this device. |
| `sermo version` | Shows the client version, the server version and the saved API version. |
| `sermo update` | Updates `sermo` to the latest release. |

`sermo` without a command needs an interactive terminal. The other commands work in all outputs, for example over SSH.

### Sign in

1. Run `sermo login`.
2. Open the address that `sermo` shows. `sermo` tries to open the browser for you.
3. Make sure that the code in the browser is the same as the code in the terminal.
4. Approve the sign-in in the browser.

`sermo` saves the session and shows your username. To cancel, push Ctrl+C.

`sermo logout` tells the server to end the session, and then deletes the saved session. If the server does not confirm, `sermo` deletes the saved session on your computer. Then you can remove the device on the security page of your account, at `<server>/account/security`.

### Flags

| Flag | Function |
| --- | --- |
| `--server URL` | Sets the server URL. |
| `--no-anim` | Stops the animations. |
| `--lang es` or `--lang en` | Sets the language. The default language is Spanish. |

### Environment variables

| Variable | Function |
| --- | --- |
| `SERMO_SERVER` | Sets the server URL. The `--server` flag has priority. |
| `SERMO_API_KEY` | Sets an API key. The API key has priority over a saved session. `sermo logout` does not change or send the API key. |
| `NO_COLOR` | Removes the colors. |
| `SERMO_CONFIG_DIR` | Sets the configuration directory. |

The server URL comes from the first item that has a value: the `--server` flag, `SERMO_SERVER`, the configuration file, the default server.

## Key bindings

### Global keys

| Key | Function |
| --- | --- |
| `F` | Forums |
| `M` | Messages |
| `N` | Notifications |
| `B` | Search |
| `P` | Profile |
| `?` | Help |
| `S` | Settings, from the help screen |
| `Q` | Quit |
| `Esc` or `Backspace` | Go back |
| `Ctrl+C` | Quit immediately |

The global keys do not work while a text field has the focus.

### Thread reader

| Key | Function |
| --- | --- |
| `j` and `k` | Scroll down and up |
| `Ctrl+D` and `Ctrl+U` | Scroll half a page down and up |
| `Space` | Scroll one page down |
| `]` and `[` | Go to the next or the previous post |
| `h` and `l` | Go to the previous or the next page of the thread |
| `u` | Go to the first unread post |
| `r` | Reply |
| `c` | Reply with a quote of the selected post |
| `e` | Edit your post |
| `+` | React to the post |
| `w` | Watch the thread, or stop the watch |
| `o` | Open a link by its number |
| `*` | Bookmark the post, or remove the bookmark |
| `a` | Open the profile of the author |
| `s` | Show the spoilers of the post |
| `v` | Show the poll of the thread |

### Forum list and thread list

| Key | Function |
| --- | --- |
| `Enter` | Open the selected forum or thread |
| `h` and `l` | Go to the previous or the next page |
| `t` | Start a new thread |
| `x` | Show only the threads with a prefix |
| `a` | Mark the forum as read. On the forum list, mark all forums as read. |
| `w` | Watch the forum, or stop the watch |

### Composer

| Key | Function |
| --- | --- |
| `Ctrl+B` | Bold text |
| `Ctrl+I` | Italic text |
| `Ctrl+U` | Underlined text |
| `Ctrl+K` | Link |
| `Ctrl+Q` | Quote |
| `Ctrl+P` | Show or hide the preview |
| `Ctrl+S` | Send |
| `Esc` | Close. If you changed the text, `sermo` asks before it discards the text. |

Posts use BB code. Each screen shows its keys in the help screen.

## Themes and effects

Change the theme and the effect in the settings screen. `sermo` saves the change immediately.

| Theme | Description |
| --- | --- |
| Habana neón | The default theme. Pink, violet and cyan neon colors on a dark background. |
| Ámbar CRT | Amber text, as on an old amber monitor. |
| Fósforo verde | Green text, as on an old green phosphor monitor. |
| Malecón | Light blue text with orange accents, as the sea wall of Havana at sunset. |

The splash screen shows 1 of 5 animated effects. The default effect is a random effect.

| Effect | Description |
| --- | --- |
| `plasma` | Sine waves of color that move and mix. |
| `fire` | Flames that move up from the bottom of the screen. |
| `gradient` | A diagonal band of color that moves across the screen. |
| `starfield` | 3 layers of stars that move left at different speeds over a dark nebula. |
| `crt` | A dark phosphor screen with scanlines, a bright band that rolls down, a small flicker and dark corners. |

Each effect uses the colors of the theme. The effects show only on the splash screen, the headers and the borders. The text of posts is always plain text with high contrast. With reduced motion or `--no-anim`, the effects do not move.

## Screens

These descriptions tell what you see on the screen.

- **Splash screen.** The name of the forum shows in large letters in the center. An animated effect fills the background. Under the name, a modem handshake types out, as on an old dial-up system. Push a key to go to the forums.
- **Forum list.** Categories show as section headers with a color gradient. Forums and subforums show under each category in a tree. Each row has an unread marker, the title, a short description, the thread and post counts, and the last post.
- **Thread reader.** The thread title is at the top. Each post has an author line, the date, the post number and the post text. Links have numbers. Images show as text labels. Reactions show under each post.

A status bar at the top shows the forum, your username and the time. A bar at the bottom shows the global keys.

## Configuration file

The configuration file is `config.json` in the configuration directory.

| System | Default directory |
| --- | --- |
| Windows | `%AppData%\sermo` |
| macOS | `~/Library/Application Support/sermo` |
| Linux | `~/.config/sermo` |

`SERMO_CONFIG_DIR` changes the directory.

| Field | Value |
| --- | --- |
| `server` | The server URL. |
| `theme` | The theme ID: `habana-neon`, `ambar-crt`, `fosforo-verde` or `malecon`. Empty means the default theme. |
| `effect` | The effect name. Empty means a random effect. |
| `reducedMotion` | `true` stops the animations. |
| `locale` | `es` or `en`. Empty means `es`. |

The settings screen writes this file. You can also edit it.

## Token storage

`sermo` saves the session token in the keychain of the operating system. If the keychain is not available, `sermo` saves the token in `tokens.json` in the configuration directory. Only your user can read this file. Its mode is `0600`. `sermo` keeps 1 token for each server.

## Data usage

`sermo` asks the server only for the data that a screen needs. It keeps the forum list, the site data, the thread prefixes and the reaction types in memory for the session. It asks for new notification counts 1 time each minute. `sermo` does not download images. `sermo version` reads only the start of the API document of the server.

## Development

You need Go and `golangci-lint` version 2.

| Command | Function |
| --- | --- |
| `make build` | Builds the `sermo` binary. |
| `make test` | Runs the tests. |
| `make vet` | Runs `go vet`. |
| `make lint` | Runs `golangci-lint`. |
| `make generate` | Makes the API client in `internal/api/gen` again from `api/openapi.json`. |
| `make spec` | Downloads `api/openapi.json` from the server and then runs `make generate`. To use a different server, add `SERVER=url`. |

If you do not have `make`, run the commands of each target from the `Makefile`.

Do not edit `internal/api/gen`. To use a new API operation, add it to `api/oapi-codegen.yaml` and run `make generate`.

## Releases

To make a release, push a tag that starts with `v`, for example `v1.2.0`. The release workflow runs the tests and GoReleaser. GoReleaser builds the archives and a `checksums.txt` file, and publishes them on GitHub Releases.

## License

MIT. See the `LICENSE` file.
