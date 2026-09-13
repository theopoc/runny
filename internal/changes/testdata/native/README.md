# Native summary fixtures

Captured on macOS arm64 on 2026-09-13 using Terraform 1.14.3, OpenTofu 1.12.6 and Terragrunt 1.1.4. These are real command outputs, including ANSI and Terragrunt's default pretty / JSON log envelopes. Temporary root paths were replaced with `/work`.

The configurations use only the built-in `terraform_data` resource with a literal string input. No cloud provider, credentials or remote backend is involved. Single-unit cases create one resource, apply it, then plan again. Terragrunt runs two units (`east/db`, `west/db`) with one and two resources, respectively. The failure case uses an after-plan hook running `sh -c 'exit 2'`.

`manifest.json` records commands, process exit codes and expected resource results. Terragrunt ran with `TG_TF_PATH` pointing to the fixture OpenTofu binary. `CHECKPOINT_DISABLE=1` and `TF_IN_AUTOMATION=1` were set. The JSON log capture contains engine writes; it is not the engine's `-json` mode.

Official release archives were checked against their published SHA256SUMS before use:

- OpenTofu `tofu_1.12.6_darwin_arm64.zip`: `e083ee43790ab9e19ad66d9933e24a7244a1412e1d5728f37999ae2163fdac95`
- Terragrunt `terragrunt_darwin_arm64.zip` (v1.1.4): `10722e6aade0b9476ef9853757dd4b37701ae3be6fb4a74495dd1e4718c0cb78`

Regular tests replay the fixtures without installing tools or running infrastructure commands.
