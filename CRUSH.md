# Titanic
Write a terminal UI app in Go using the Bubbletea framework. The app should read a config file (TOML or YAML) that defines one or more directory pairs, each with a source and a destination path.

For each pair:
- List all files recursively in both source and destination directories
- Sort the file paths alphabetically
- Compute MD5 hashes for each file in both directories

The UI should show two columns: left is destination files, right is source files. Each line should include the relative path and the MD5 hash. If a file is missing or different between source and destination, highlight it (e.g. using different colours).

Also:
- Allow switching between directory pairs using Tab or arrow keys
- Add a 'q' key to quit the app
- Optionally, allow triggering `rsync` from source to destination using a keybind (like 's')

Make sure the code is modular and well-documented.
