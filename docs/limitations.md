# Current limitations

- SQLite is the supported Media Server database; PostgreSQL is not implemented.
- Built-in TLS/ACME and a media relay are not implemented. Direct deployments use
  a deliberate reverse proxy or local-network endpoint.
- MFA, passkeys, and automated owner-account recovery are not implemented.
- Windows offline downloads are not implemented.
- Apple clients are not implemented. Supported consumer surfaces are Web, Android
  phone, Android TV, and native Windows Desktop.
- The Windows Server MSI bundles pinned, integrity-checked FFmpeg and FFprobe, but
  production publication still requires operator-supplied Authenticode and update
  signing credentials. Unsigned artifacts are development builds only.
- Image-based subtitle burn-in and automatic multi-rendition ABR remain outside
  the current playback contract.
- Connect is optional. During a Connect outage, previously authenticated local
  access continues; explicit global sign-out removes account-linked credentials
  and offline downloads from that device for privacy.
