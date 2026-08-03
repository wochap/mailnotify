# mailnotify

A small Go program that notifies when mail has arrived in your mail directory.

Both mbsync (`<account>/INBOX/new`) and lieer (`<account>/mail/cur`, delivered
via `<account>/mail/tmp`) directory layouts are supported.

## Usage

```
mailnotify [flags] ~/path/to/mail/dir
```

### Flags

- `-u, --urgency=LEVEL`: urgency level (`low`, `normal`, `critical`). Default:
  not set, the notification server decides.
- `-t, --expire-time=TIME`: timeout in milliseconds at which the notification
  expires. `-1` (default) lets the notification server decide, `0` never
  expires.
- `-a, --app-name=APP_NAME`: app name for the notification. Default: `Mail`.
- `-i, --icon=ICON`: icon filename or stock icon to display.
- `-n, --app-icon=ICON`: application icon filename or app icon name. The
  server may or may not display it. Shares a destination with `--icon`; the
  last one on the command line wins.
- `-h, --hint=TYPE:NAME:VALUE`: extra data to attach to every notification,
  in notify-send format. Valid types are `boolean`, `int`, `double`, `string`,
  `byte` and `variant`. Repeatable. Example:
  `--hint=string:custom-sound:message`. Hints are applied after the other
  flags, so they can override e.g. the urgency.

Example:

```
mailnotify --urgency=critical --hint=string:custom-sound:message ~/mail
```

## Contributing

If you want to contribute to this project, submit an issue or pull request.
