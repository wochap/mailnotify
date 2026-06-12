{
  description = "mailnotify";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-parts.url = "github:hercules-ci/flake-parts";
  };

  outputs =
    inputs@{
      self,
      nixpkgs,
      flake-parts,
    }:
    (flake-parts.lib.mkFlake { inherit inputs; } {
      systems = [ "x86_64-linux" ];
      perSystem =
        {
          pkgs,
          system,
          ...
        }:
        {
          _module.args.pkgs = import inputs.nixpkgs { inherit system; };

          packages = rec {
            mailnotify = pkgs.buildGoModule {
              pname = "mailnotify";
              version = "unstable-2026-06-12";
              src = self;

              propagatedBuildInputs = [ pkgs.libnotify ];

              vendorHash = "sha256-UQVGX9G//ksoNJTQtVFJLiIL5z97wChNgAwP+DN/mus=";
            };
            default = mailnotify;
          };

          devShells.default = pkgs.mkShell {
            buildInputs = with pkgs; [
              go
              libnotify
              pre-commit
            ];
          };
        };
    });
}
