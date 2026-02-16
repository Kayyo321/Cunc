# Cunc

**C**onventionally **Unc**onventional emailing system designed for the Linux terminal

---

## About

**Cunc** is a powerful, terminal-based email client for Linux that brings the efficiency of command-line interfaces to email management. Built with Go and the Bubble Tea TUI framework, Cunc offers a sleek, keyboard-driven interface for reading, composing, and managing emails without leaving your terminal.

### Key Features

- **IMAP Email Fetching** - Connect to your email provider and fetch emails directly
- **Rich Email Composition** - Write emails with spell checking and typo detection
- **Draft Management** - Save and resume email drafts
- **Attachment Support** - Add, view, and download email attachments
- **Contact Autocomplete** - Quick recipient suggestions based on email history
- **Customizable Settings** - Tailor Cunc to your preferences
- **Beautiful TUI** - Clean, animated interface with Unicode support

---

## Installation

### Download from GitHub Releases

1. Visit the [Cunc releases page](https://github.com/yourusername/Cunc/releases)
2. Download the latest release binary for Linux (e.g., `cunc-linux-amd64`)
3. Make the binary executable:
   ```bash
   chmod +x cunc-linux-amd64
   ```
4. Optionally rename it:
   ```bash
   mv cunc-linux-amd64 cunc
   ```

### Adding Cunc to Your PATH

To run `cunc` from anywhere in your terminal, add it to your system PATH:

#### Option 1: Move to a directory already in PATH
```bash
sudo mv cunc /usr/local/bin/
```

#### Option 2: Add a custom directory to PATH
```bash
# Create a directory for your binaries
mkdir -p ~/.local/bin

# Move cunc to this directory
mv cunc ~/.local/bin/

# Add to your shell configuration file
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc

# Reload your shell configuration
source ~/.bashrc
```

> **Note:** If you use `zsh`, replace `~/.bashrc` with `~/.zshrc`

Verify the installation:
```bash
cunc -h
```

THEN ... Hookup your email account!! (see troubleshooting below if you have 2FA)

---

## Usage

### Running Cunc

Cunc can be launched in two modes:

1. **Interactive Director Mode** (recommended):
   ```bash
   cunc
   ```
   This launches a visual menu where you can navigate between Inbox, Compose, Drafts, and Settings.

2. **Direct Mode** (with command-line arguments):
   ```bash
   cunc [option]
   ```

### Command-Line Arguments

| Argument | Alias | Description |
|----------|-------|-------------|
| `-help` | `-h` | Display help information |
| `-compose` | `-c` | Open the email composer |
| `-drafts` | `-d` | View and edit saved drafts |
| `-settings` | `-s` | Open settings configuration |
| `-inbox` | `-i` | View your email inbox |

**Examples:**
```bash
# Launch interactive mode
cunc

# Go directly to compose a new email
cunc -c

# Check your inbox
cunc --inbox

# Manage drafts
cunc -d
```

---

## Settings

Cunc stores its configuration in `~/.local/share/cunc/`. Settings are divided into:
- `settings.json` - General preferences
- `sensitive.json` - Credentials (stored with restricted permissions)

### Accessing Settings

Launch the settings interface:
```bash
cunc -s
```

Navigate with **↑/↓**, edit values directly, then press **Ctrl+S** to save.

### Available Settings

#### Account

| Setting | Description | Required |
|---------|-------------|----------|
| **email** | Your email address (e.g., `user@gmail.com`) | Yes |
| **password** | Your email password | If no 2FA |
| **2fa app-password** | App-specific password for 2FA-enabled accounts | If using 2FA |

> **Gmail Users:** Enable IMAP in Gmail settings and use an [App Password](https://support.google.com/accounts/answer/185833) if 2FA is enabled.

#### Preferences

| Setting | Default | Options | Description |
|---------|---------|---------|-------------|
| **emails per page** | `40` | Number | Number of emails to display per page in inbox |
| **should delete draft on send** | `n` | `y` / `n` | Automatically delete draft after sending |
| **max typos displayed** | `3` | Number | Maximum number of typos to show in spell check |
| **prevent send with typos** | `y` | `y` / `n` | Block sending if typos are detected |
| **unicode support** | `y` | `y` / `n` | Display emoji and Unicode characters in UI |

#### Paths

| Setting | Default | Description |
|---------|---------|-------------|
| **default attachment download path** | `~/Downloads` | Where to save downloaded attachments |
| **drafts directory** | `~/.local/share/cunc/drafts` | Directory where email drafts are saved |

---

## Keyboard Shortcuts

### Director (Main Menu)

| Key | Action |
|-----|--------|
| `↑` / `↓` | Navigate options |
| `Enter` | Select option |
| `Ctrl+Q` | Quit |

### Inbox

| Key | Action |
|-----|--------|
| `↑` / `k` | Move up |
| `↓` / `j` | Move down |
| `h` / `←` | Previous page |
| `l` / `→` | Next page |
| `Enter` | View selected email |
| `Ctrl+R` | Refresh inbox |
| `Ctrl+Q` | Quit to main menu |

**While viewing an email:**

| Key | Action |
|-----|--------|
| `a` | View attachments |
| `Esc` | Back to inbox list |
| `Ctrl+Q` | Quit |

**While viewing attachments:**

| Key | Action |
|-----|--------|
| `↑` / `k` | Move up |
| `↓` / `j` | Move down |
| `d` | Download selected attachment |
| `Esc` | Back to email |

### Compose / Editor

| Key | Action |
|-----|--------|
| `Tab` | Navigate between fields (To, Subject, Body) |
| `Shift+Tab` | Navigate backwards |
| `Ctrl+A` | Add attachment |
| `Ctrl+F` | Manage attachments (view/delete) |
| `Ctrl+S` | Save as draft |
| `Ctrl+Y` | Send email |
| `Ctrl+Q` | Quit (without sending) |

**Features:**
- **Contact Autocomplete**: Start typing an email address, and Cunc will suggest contacts from your history
- **Spell Check**: Typos are detected automatically while composing (configurable)

### Drafts

| Key | Action |
|-----|--------|
| `↑` / `k` | Move up |
| `↓` / `j` | Move down |
| `Enter` | Open/edit selected draft |
| `d` | Delete draft (press `y` to confirm, `n` to cancel) |
| `Ctrl+Q` | Quit to main menu |

### Settings

| Key | Action |
|-----|--------|
| `↑` / `↓` | Navigate settings |
| Type | Edit the focused field |
| `Ctrl+S` | Save settings |
| `Ctrl+Q` | Quit without saving |

---

## Configuration Tips

### First-Time Setup

1. Run `cunc -s` to open settings
2. Enter your **email** address
3. Enter your **password** or **2fa app-password**
4. Adjust preferences as desired
5. Press **Ctrl+S** to save

### Gmail Configuration

For Gmail users:
1. Enable IMAP: Gmail Settings → Forwarding and POP/IMAP → Enable IMAP
2. If using 2FA (recommended):
   - Generate an App Password: [Google Account](https://myaccount.google.com/) → Security → 2-Step Verification → App passwords
   - Use the app password in the `2fa app-password` field in Cunc settings

### Other Email Providers

Cunc uses IMAP. Ensure your email provider:
- Supports IMAP
- Has IMAP enabled for your account
- Allows third-party app access (may require an app-specific password)

---

## Troubleshooting

**Can't connect to email server:**
- Verify your email and password are correct in settings
- Check if IMAP is enabled for your email account
- For Gmail, ensure you're using an App Password if 2FA is enabled

**Cunc command not found:**
- Ensure the binary is in your PATH
- Try running with the full path: `~/.local/bin/cunc`

**No emails showing:**
- Press `Ctrl+R` in the inbox to refresh
- Verify your credentials in settings
- Check your internet connection

---

## License

Cunc is licensed under the terms specified in the [LICENSE](LICENSE) file.

---

## Contributing

Contributions, issues, and feature requests are welcome! Feel free to add stuff to the issues.

---

## Author

Kayyo321/Sully 2026
Created with <3 for terminal enthusiasts everywhere.

---

**Happy Emailing! :)**
