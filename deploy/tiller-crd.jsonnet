// CRD and tiller, with controller running as a sidecar

local utils = import "utils.libsonnet";
local tiller = import "tiller.jsonnet";

// Run CRD controller as a sidecar, and restrict tiller port to pod-only
local controller_overlay = {
  spec+: {
    template+: {
      spec+: {
        volumes+: [
          // Used as temporary space while downloading charts, etc.
          {name: "home", emptyDir: {}},
        ],
        containers: [
          super.containers[0] {
            assert self.name == "tiller",
            // Nuke exposed tiller port
            ports: [], // Informational only
            command: ["/tiller"],
            args+: ["--listen=localhost:44134"],  // Restrict to pod only
          },
          {
            name: "controller",
            image: "bitnami/helm-crd-controller:latest",
            securityContext: {
              readOnlyRootFilesystem: true,
            },
            command: ["/controller"],
            args: [
              "--home=/helm",
              "--host=localhost:44134",
            ],
            env: [
              {name: "TMPDIR", value: "/helm"},
            ],
            volumeMounts: [
              {name: "home", mountPath: "/helm"},
            ],
          },
        ],
      },
    },
  },
};

{
  crd: utils.CustomResourceDefinition("helm.bitnami.com", "v1", "HelmRelease"),
  crd: utils.CustomResourceDefinition("helm.bitnami.com", "v1", "HelmRelease") {
    spec+: {
      names+: {shortNames: ["hr", "release"]},
      validation: {
        openAPIV3Schema: {
          "$schema": "http://json-schema.org/draft-04/schema#",
          description: "Helm chart release",
          type: "object",
          properties: {
            spec: {
              type: "object",
              properties: {
                repoUrl: {
                  type: "string", // URL
                },
                chartName: {
                  type: "string",
                },
                version: {
                  type: "string",
                },
                values: {
                  type: "string", // YAML
                },
              },
            },
          },
        },
      },
    },
  },

  tiller: tiller + controller_overlay,
}
