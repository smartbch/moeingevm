 
SUBDIRS = intx evmone sha256 ripemd160 keccak/src host_bridge

.PHONY: all
all:
	@list='$(SUBDIRS)'; for subdir in $$list; do \
		echo "Clean in $$subdir";\
		$(MAKE) -C $$subdir;\
	done

.PHONY: clean
clean:
	@echo Making clean
	@list='$(SUBDIRS)'; for subdir in $$list; do \
		echo "Clean in $$subdir";\
		$(MAKE) -C $$subdir clean;\
	done
