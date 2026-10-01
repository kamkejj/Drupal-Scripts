# Dropkit

Dropkit creates local Drupal projects on macOS with DDEV. It supports Drupal Core, Drupal Commerce, and Drupal CMS.

## Install

From this repository, run:

```bash
./install.sh
```

## Create a project

Go to the directory where you want the project created, then choose a guided installer:

```bash
cd ~/Projects
dropkit install   # Drupal Core
dropkit commerce  # Drupal Commerce
dropkit cms       # Drupal CMS
```

The installer asks for a project name and container runtime. Drupal CMS also requires completing setup in the browser.

## Use without prompts

Create a plan, review it, then authorize and apply it:

```bash
dropkit install plan --name my-site --parent "$PWD" --provider colima --drupal-version 11 --output json > plan.json
dropkit install apply --plan plan.json --allow-network --allow-host-changes
dropkit install verify --plan plan.json
```

Use `commerce` instead of `install` for Drupal Commerce (version 10 or 11), or `cms` for Drupal CMS (omit `--drupal-version`). Run `dropkit help` or `dropkit help install` for more options.
