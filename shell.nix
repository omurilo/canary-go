{ pkgs ? import <nixpkgs> {} }:

let
  podmanHelpers = pkgs.lib.makeBinPath [
    pkgs.gvproxy
    pkgs.netavark
    pkgs.aardvark-dns
    pkgs.conmon
    pkgs.crun
    pkgs.slirp4netns
    pkgs.passt
    pkgs.virtiofsd
    pkgs.qemu
  ];
in

pkgs.mkShell {
  packages = with pkgs; [
    # Docker compatibility
    docker-client
    docker-compose

    # Podman
    podman

    # OCI runtime
    crun
    conmon

    # Networking
    netavark
    aardvark-dns
    slirp4netns
    passt

    # VM support (podman machine)
    qemu
    virtiofsd
    gvproxy

    # Storage
    fuse-overlayfs

    # Utilities
    jq
    curl
    git
  ];

  shellHook = ''
    export TMPDIR=/tmp/podman-build
    mkdir -p "$TMPDIR"

    chmod 1777 "$TMPDIR"
    export PATH="${podmanHelpers}:$PATH"

    mkdir -p ~/.config/containers

    cat > ~/.config/containers/policy.json <<EOF
{
  "default": [
    {
      "type": "insecureAcceptAnything"
    }
  ]
}
EOF

   cat > ~/.config/containers/containers.conf <<EOF
[engine]
runtime = "crun"

helper_binaries_dir = [
  "${pkgs.podman}/libexec/podman",
  "${pkgs.gvproxy}/bin",
  "${pkgs.netavark}/bin",
  "${pkgs.aardvark-dns}/bin",
  "${pkgs.conmon}/bin",
  "${pkgs.crun}/bin",
  "${pkgs.slirp4netns}/bin",
  "${pkgs.passt}/bin"
]

events_logger = "file"

[network]
default_rootless_network_cmd = "pasta"
EOF

    # Compatibilidade docker CLI -> podman
    export DOCKER_HOST="unix://$XDG_RUNTIME_DIR/podman/podman.sock"

    echo "=== Ambiente Podman pronto ==="
    podman --version
    docker --version || true
    qemu-system-x86_64 --version | head -1 || true

    echo ""
    echo "Socket Docker:"
    echo "$DOCKER_HOST"
    echo ""
    echo "Para iniciar:"
    echo "  podman system service --time=0 $DOCKER_HOST &"
  '';
}
