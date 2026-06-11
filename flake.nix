{
  description = "output-monitor: split TUI panes showing all logs + filtered matches";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};

        output-monitor = pkgs.buildGoModule {
          pname = "output-monitor";
          version = "0.1.0";
          src = ./.;
          # vendored deps — no hash needed
          vendorHash = null;

          meta = with pkgs.lib; {
            description = "Split-pane TUI: all logs + filtered matches from piped stdout";
            homepage = "https://github.com/rhousand/output-monitor";
            license = licenses.mit;
            mainProgram = "output-monitor";
          };
        };
      in
      {
        packages = {
          default = output-monitor;
          inherit output-monitor;
        };

        apps = {
          default = {
            type = "app";
            program = "${output-monitor}/bin/output-monitor";
          };
          output-monitor = {
            type = "app";
            program = "${output-monitor}/bin/output-monitor";
          };
        };

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [ go gopls gotools ];
        };
      }
    ) // {
      nixosModules.default = import ./nix/module.nix self;
      nixosModules.output-monitor = import ./nix/module.nix self;

      overlays.default = final: _prev: {
        output-monitor = self.packages.${final.system}.default;
      };
    };
}
