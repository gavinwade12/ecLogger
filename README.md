# ecLogger

## What

ecLogger is a desktop application for connecting to a Subaru ECU via the SSMII protocol and logging data.

## Why

Subarus.. Programming.. Data.. Something new to learn.. I'm a nerd. I would also like a few-seconds warning before my WRX blows up.

## How

ecLogger is written in [Go](https://go.dev/) and uses [Fyne](https://fyne.io/) for the GUI layer. 
This allows it to run just about anywhere.

## Building

Because Fyne is used, you must first satisfy any of its [prerequisites](https://developer.fyne.io/started/#prerequisites) for your system. You can then build the logger UI with go build:

    go build -o cmd/logger-ui/logger-ui cmd/logger-ui/*.go

This will place an executable named `logger-ui` in the `cmd/logger-ui/` directory.