# Libraries and filesystem sources

A library is VyNode configuration for `MOVIES` or `TV` physical media and owns one or more independent filesystem sources. Physical files and streams remain separate from future logical movies, shows, seasons, and episodes.

Sources must be absolute, existing, readable server-visible directories. Paths are cleaned, root symlinks resolved, duplicates rejected, and paths overlapping VyNode configuration or transcode storage forbidden. Scans do not follow directory or file symlinks. VyNode provides no remote filesystem browser.

Docker and Unraid store container-visible paths: a host `/mnt/user/media/movies` mounted at `/media/movies` is configured as `/media/movies`. Native Windows uses absolute paths such as `D:\Movies`; UNC support is not claimed as validated.

Deleting a library or source removes only VyNode configuration and inventory. It never changes user media. If a source root is unavailable, its prior inventory is preserved. Individual files become `MISSING` only after a successful root walk; reappearance restores `AVAILABLE` and retains the ID at the same source-relative path.
