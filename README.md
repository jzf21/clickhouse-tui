# clickhouse-tui

A terminal user interface for managing ClickHouse instances — both local and cloud.

Built with Go and [Bubbletea](https://github.com/charmbracelet/bubbletea).

## Features

- **Local connections** — add, list, and delete ClickHouse connection profiles (host, port, user, database)
- **Service control** — start and stop local ClickHouse servers (native or Docker)
- **Cloud integration** — authenticate with ClickHouse Cloud, view services across organizations, and start/stop them remotely
- **Persistent config** — connections and credentials stored in `~/.clickhouse-tui/connections.json`

## Install

```sh
go build -o clickhouse-tui .
```

## Usage

```sh
./clickhouse-tui
```

### Keybindings

| Key | Action |
|-----|--------|
| `Tab` | Switch between Local / Cloud tabs |
| `j` / `k` | Navigate up / down |
| `a` | Add a new connection |
| `d` | Delete selected connection |
| `f` | Filter which cloud services are visible |
| `s` | Start / stop a service |
| `Enter` | View connection details |
| `q` / `Ctrl+C` | Quit |

## License

MIT
