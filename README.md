# regclean

Finds orphaned registry entries in Windows (Run/RunOnce, Uninstall, App Paths)
and cleans them up. The default is a dry run.

    go mod tidy
    go build -o regclean.exe ./cmd/regclean
    go test ./...            # Also runs on Linux/macOS (fake registry)

    regclean                          # view only
    regclean -v                       # including entries that are intact or cannot be verified
    regclean -areas run,apppaths      # Narrow Down Categories
    regclean -scope user              # HKCU only; no admin privileges required
    regclean -delete                  # Delete; asks [y/n/a/q] for each entry
    regclean -delete -yes             # without being asked
    regclean -delete -backup D:\bk    # Custom backup folder

Before any deletion, the affected key is exported as a .reg file; if this
fails, nothing is deleted. To restore: double-click the .reg file or use `reg import`.

Only items that are definitely missing are considered orphaned. Offline drives, network paths,
interpreter calls (cmd, powershell, msiexec, ...), MSI entries, and names without a path
are never touched.
