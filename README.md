<div align="center">
  
# 🌌 Terminas IDE
**The Next-Generation, AI-Powered Terminal IDE**

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

*A lightning-fast, fully portable software development environment built entirely in Go. Seamlessly fusing a modern text editor, an integrated command console, and an omniscient AI assistant into a single, cohesive executable.*

</div>

---

## ✨ Features

- **⚡ Blazing Fast & Lightweight**: Written in pure Go (`bubbletea`), Terminas compiles down to a single zero-dependency executable. No Node modules, no Electron bloat. 
- **🤖 Omniscient AI Assistant**: Built-in support for Claude, ChatGPT, Gemini, and DeepSeek. The AI has deep context awareness of your entire workspace and can automatically create, edit, and read files in background.
- **🎨 Modern "Dracula" Design**: Carefully crafted CLI aesthetics. Featuring reactive status bars, dynamic language icons, and beautiful layout management right in your terminal.
- **💻 Integrated OS Terminal**: Toggle instantly between your code editor and a fully functional interactive shell (`Ctrl+T`). Execute commands, compile code, and run scripts without ever leaving the IDE.
- **📁 Smart File Explorer**: Browse your project tree visually. Create, read, and delete files with integrated commands.
- **⌨️ Vim-Inspired Modality**: Fluid `NORMAL`, `INSERT`, and `COMMAND` modes for frictionless keyboard-driven navigation.

## 🚀 Quick Start

### Installation

No complex setup. Just download and run.

```bash
# Clone the repository
git clone https://github.com/spiccoaura/Terminas.git

# Navigate to the project directory
cd Terminas

# Run the installer script (Windows)
.\install.ps1
```
*This will compile `terminas.exe` and add it to your global PATH.*

### Usage
Open any terminal and type:
```bash
terminas
```

## 🛠️ Keybindings

- `Ctrl + T` : Instantly toggle between Editor and Console mode.
- `Ctrl + O` : Toggle sidebar (Switch between File Explorer and AI Chat).
- `Ctrl + W` : Toggle AI Workspace Context (ON = Reads whole project, OFF = Reads current file only).
- `Ctrl + S` : Quick Save current file.
- `Ctrl + C` : Quit Terminas.

## 💬 The AI Console

Once inside the IDE, set up your AI API key to unlock the magic:
```bash
/login sk-your-api-key-here
```

Then simply ask the AI to code for you:
```bash
/ai create a snake game in python
```
*Watch as the AI generates the files, creates directories, and opens them in your editor automatically!*

## 🤝 Contributing

Contributions, issues, and feature requests are welcome! 
Feel free to check the [issues page](https://github.com/spiccoaura/Terminas/issues).

## 📜 License

This project is [MIT](LICENSE) licensed.
