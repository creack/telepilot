# TODO: Consider pinning versions/checksums.
IMAGES = alpine busybox

.PHONY: images
images: ${IMAGES:%=images/%}

images/%: images/%/.done
	true

images/%/.done: images/%.tar
	mkdir -p $(dir $@)
	tar xf $< -C $(dir $@)
	touch $@

images/%.tar:
	mkdir -p $(dir $@)
	docker rm -f -v telepilot-imgdump
	docker run --name telepilot-imgdump $* true
	docker export telepilot-imgdump > $@
	docker rm -f -v telepilot-imgdump

.PHONY: clean-images
clean-images:
	-docker rm -f -v telepilot-imgdump 2> /dev/null
	rm -rf images
clean: clean-images
