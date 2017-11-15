KUBECFG = kubecfg

LIBFILES = tiller.jsonnet utils.libsonnet

all: tiller-crd.yaml

%.yaml: %.jsonnet $(LIBFILES)
	$(KUBECFG) show $< > $@.tmp
	mv $@.tmp $@

clean:
	$(RM) *.yaml

.PHONY: all clean
