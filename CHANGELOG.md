# CHANGE LOG

## v260203

- ui: responsive terminal user interface
- feature: list all installed packages
- ui: search input field
- ui: list of installed packages
- ui: command bar (hints)
- feature: supported package managers are:
  - apt, dpkg, dpkg-query
  - snap
  - flatpak
  - pacman
  - nix-env
  - brew (homebrew)
  - port (macports)
  - dnf, rpm
  - guix
- status bar (e.g. updating local index...)
- search for a package with all available package managers
- feature: install package
- feature: uninstall package

## next

- fix: de-duplicate packages when searching. show a package only once.
- fix: if search input field is empty, show all installed packages
- feature: update/upgrade package (using Enter similar to INSTALL)
- feature: `/quit` or `/q` to quit
- feature: `/help` or `/h` to show help
- feature: `/selfupdate` to update lazyinstaller
- feature: `/selfuninstall` to uninstall lazyinstaller
- feature: `/version` or `/v` to show version
- feature: `/up` to list installed packages that can be upgraded
- script to install lazyinstaller
- script to uninstall lazyinstaller
- script to build lazyinstaller for different operating systems and architectures
