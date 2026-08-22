# Hardware acceleration

VyNode models software, NVIDIA, Intel QSV, VA-API, AMD AMF, and VideoToolbox backends with separate detection, encoder, and decoder state. FFmpeg encoder presence and relevant platform/device exposure are probed and reported independently.

Software `libx264` is the only Phase 6 validated backend. Hardware entries remain unavailable until a real initialization test succeeds; device-node or encoder presence alone never marks a backend available. Auto therefore selects software in the validated image.

Linux Intel/VA-API deployments generally expose `/dev/dri`. NVIDIA deployments use host drivers and the NVIDIA container runtime. Proprietary drivers are not bundled. AMD AMF and VideoToolbox remain architectural targets. Unraid uses the same image and host device mappings.

Status in the Phase 6 development environment: NVIDIA not detected; Intel QSV not runtime validated; VA-API not runtime validated; AMD AMF not detected; VideoToolbox not detected.
