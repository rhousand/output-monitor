self:
{ config, lib, pkgs, ... }:

let
  cfg = config.programs.output-monitor;
in
{
  options.programs.output-monitor = {
    enable = lib.mkEnableOption "output-monitor split-pane log viewer";
  };

  config = lib.mkIf cfg.enable {
    environment.systemPackages = [
      self.packages.${pkgs.stdenv.hostPlatform.system}.default
    ];
  };
}
