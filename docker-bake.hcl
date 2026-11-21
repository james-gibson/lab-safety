group "default" {
  targets = ["wan-ingress", "seed-invite", "seed-announce"]
}

target "_common" {
  context    = "."
  dockerfile = "Dockerfile"
  platforms  = ["linux/amd64", "linux/arm64"]
}

target "wan-ingress" {
  inherits = ["_common"]
  target   = "wan-ingress"
  tags     = ["lab-safety/wan-ingress:local"]
  output   = ["type=oci,dest=dist/wan-ingress.oci.tar"]
}

target "seed-invite" {
  inherits = ["_common"]
  target   = "seed-invite"
  tags     = ["lab-safety/seed-invite:local"]
  output   = ["type=oci,dest=dist/seed-invite.oci.tar"]
}

target "seed-announce" {
  inherits = ["_common"]
  target   = "seed-announce"
  tags     = ["lab-safety/seed-announce:local"]
  output   = ["type=oci,dest=dist/seed-announce.oci.tar"]
}
