## Which file do I download?

| Your computer | File |
| --- | --- |
| Windows (almost every PC) | `dgg-obs-tools_windows_x64.exe` |
| Windows on an ARM laptop (Snapdragon) | `dgg-obs-tools_windows_arm64.exe` |
| Mac with Apple silicon (M1 or later) | `dgg-obs-tools_macos_arm64.tar.gz` |
| Mac with an Intel processor | `dgg-obs-tools_macos_x64.tar.gz` |
| Linux | `dgg-obs-tools_linux_x64.tar.gz` (or `_arm64`) |

Not sure? On Windows, Settings → System → About shows "x64-based" or
"ARM-based" next to System type. On a Mac, Apple menu → About This Mac shows
"Chip: Apple M…" for Apple silicon.

The files aren't signed, so the first run asks once: on Windows click
**More info** → **Run anyway**; on a Mac right-click the file and choose
**Open**. After that it updates itself. See the
[README](https://github.com/StrawWaffle/dgg-obs-tools#readme) for setup.

`checksums.txt` is what the app checks updates against; you don't need it.
