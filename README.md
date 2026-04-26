# Faceit Analyzer

A Go web app that analyzes a Faceit CS2 player's match history and surfaces insights the official site doesn't provide.

## Features

- K/D trend chart with 30/60/90 day toggle
- Per-map win rate, K/D, and headshot breakdown (sortable)
- Match pattern cards: win rate, avg K/D, HS rate, recent form
- Auto-generated weak spots and performance insights
- SQLite cache — first load ~20s, subsequent loads instant

## Prerequisites

- [Go 1.22+](https://go.dev/dl/)
- A Faceit API key — create one at [developers.faceit.com](https://developers.faceit.com) under **API Keys** (choose Server-side)

## Running locally

```bash
git clone https://github.com/YOUR_USERNAME/faceit-analyzer.git
cd faceit-analyzer
go mod tidy