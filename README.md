<p align="center">
  <img src="docs/logo/logo.svg" alt="ecLogger logo" width="128">
</p>

# ecLogger

A cross-platform desktop app for logging, graphing and diagnosing Subaru ECUs over the SSM2 protocol.

Plug in a serial cable, pick the parameters you care about, and watch them live, log them to CSV, and graph a log afterwards. It's built to run just as well on a laptop as on a Raspberry Pi touchscreen mounted in the car.

![Graphing a log file, with the cursor reading every lane at one moment](docs/screenshots/graphing.png)

## Features

- **Live logging.** Choose from hundreds of ECU parameters, plus derived ones such as boost, and see their current, min and max values on live tiles.
- **Log to file.** Record any set of parameters to CSV. Log file names are templated, so each log can be named by ROM ID and timestamp.
- **Graphing.** Chart parameters as stacked lanes on a shared time axis, either live while connected or from any saved log. Zoom, pan, and hover (or tap) to read every lane's value at that moment.
- **Log files.** Browse, graph and delete your logs without leaving the app.
- **Unit conversion.** Pick the unit for each parameter (psi, kPa, bar, °F, °C, mph, km/h, …) and it's used on screen and in your logs.
- **Trouble codes.** Read the set and stored DTCs from the ECU.
- **Remembers your setup.** Selected parameters, units, port and settings are saved between runs. It can connect automatically on start and open straight to the Logging tab.
- **A CLI too.** `ssm2-cli` does headless logging, handy for scripting or a Pi without a screen.

![Live logging tiles showing the current, min and max of each parameter](docs/screenshots/logging.png)

![Choosing parameters to log](docs/screenshots/parameters.png)

## Getting started

### What you need

- A Subaru with an SSM2-compatible ECU.
- A cable that exposes the ECU's diagnostic line as a serial port on your computer.
- [Go](https://go.dev/dl/) 1.25 or newer to build the app.

There are no prebuilt binaries yet, so for now you build it from source.

### Build

ecLogger's UI uses [Fyne](https://fyne.io/), so first install its [prerequisites](https://developer.fyne.io/started/#prerequisites) for your OS (mostly a C compiler and some graphics headers). Then:

```sh
git clone https://github.com/gavinwade12/ecLogger.git
cd ecLogger

go build -o cmd/logger-ui/logger-ui ./cmd/logger-ui
go build -o cmd/ssm2-cli/ssm2-cli ./cmd/ssm2-cli
```

### Use it

1. Run `cmd/logger-ui/logger-ui`.
2. On the **Connection** tab, pick your serial port and connect.
3. On **Parameters**, tick *Live Log* for what you want to watch and *Log To File* for what you want to record.
4. Open **Logging** for live tiles and to start and stop a log file, or **Graphing** to chart values as they arrive.
5. Come back to **Log Files** to graph or delete past logs.

Settings and logs live in your home directory by default; the log folder and file name format can be changed in **Settings**.

### Try it without a car

A dev build adds a *Use Fake Connection* switch in Settings that feeds the app made-up data. It's good for poking around or working on the UI.

```sh
go build -tags dev -o cmd/logger-ui/logger-ui ./cmd/logger-ui
```

## Command line

```sh
ssm2-cli ports list                       # find your serial port
ssm2-cli ports set                        # save the port to use
ssm2-cli add_param --paramID <id> --unit <unit>
ssm2-cli log                              # log the configured parameters
```

Run `ssm2-cli --help` for everything else.

## Project layout

| Path | What's there |
| --- | --- |
| `cmd/logger-ui` | The Fyne desktop app |
| `cmd/ssm2-cli` | The command-line logger |
| `protocols/ssm2` | The SSM2 protocol: packets, parameters, DTCs, and the connection |
| `internal/logging` | Logging sessions, CSV reading and writing, and chart data |
| `internal/serialport` | Serial port access |
| `units` | Unit definitions and conversions |

Tests run with `go test ./...`.

## Safety

ecLogger only reads from the ECU. Even so, it's a hobby project that comes with no warranty: don't watch the screen instead of the road, and make sure a passenger or a mounted display is doing the looking when you're moving.

## Why

Subarus, programming, data, and something new to learn. Also, a few seconds of warning before my WRX blows up would be nice, and it gave me a reason to finally buy a touchscreen for my Raspberry Pi.

## License

[BSD 3-Clause](LICENSE)
