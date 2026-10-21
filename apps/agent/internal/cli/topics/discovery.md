# Discovery

Orion looks at this server and **suggests** what to monitor. It never changes anything on its own.

## What it looks at

- **services**: systemd units (Linux) or launchd jobs (macOS)
- **containers** and **pm2** apps
- **ports** that are listening
- **disks** and mounts
- **logs**: log files it can read
- **storage**: directories and Docker volumes worth sizing

## Commands

- `orion-agent discover` lists the suggestions.
- `orion-agent setup` chooses what to accept.
- `orion-agent discover dismiss <id>` hides one; `restore <id>` brings it back.
- `orion-agent discover ignore <id>` never suggests it again (saved to `discovery.ignore`).

## Settings

```yaml
discovery:
  enabled: true
  kinds: [services, containers, ports]
  ignore: [port:5432]
  report: true
```

`report: false` keeps the scan on this server. Only names are reported, never file contents.
