# Alloy

**Alloy** is a fast, opinionated package manager that installs software directly onto your system, prioritizing speed, breadth, and correctness over isolation.

It aims to answer a simple question:

> Why can’t one tool install *everything* quickly — and remove it cleanly?

Alloy installs packages natively, tracks every file it touches, and guarantees complete removal on uninstall. No sandboxes, no virtual filesystems, no long-lived background services — just fast installs and a clean system.

---

## Design Principles

- **Speed first**  
  Installs should be limited by network and disk, not dependency solvers or abstraction layers.

- **Broad coverage**  
  System packages, CLI tools, language runtimes, and single binaries should all be installable through one interface.

- **Native by default**  
  Packages are installed directly onto the host system, using conventional locations.

- **Strict uninstall hygiene**  
  Every file created, modified, or removed during install is recorded. Uninstall means *nothing left behind*.

- **Opinionated, minimal UX**  
  Few commands. Predictable behavior. No endless configuration.

---

## What Alloy Is (and Is Not)

**Alloy is:**
- A general-purpose package manager
- System-native and fast
- Focused on install → use → remove lifecycle correctness
- Designed to work well for individuals and small teams

**Alloy is not:**
- A container runtime
- A source-based distribution
- A full replacement for every ecosystem’s build tools
- A sandboxing or isolation system

---

## Basic Usage

```bash
alloy install ripgrep
alloy install node
alloy install postgresql

alloy list

alloy remove ripgrep

