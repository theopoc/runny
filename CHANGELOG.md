# Changelog

## [0.3.0](https://github.com/theopoc/runny/compare/v0.2.0...v0.3.0) (2026-08-14)


### Features

* **tui:** add mouse wheel pane scrolling ([#46](https://github.com/theopoc/runny/issues/46)) ([0885150](https://github.com/theopoc/runny/commit/08851508289722ed27fc104fcf62c0d7264acd6d))
* **tui:** focus panes with mouse clicks ([#45](https://github.com/theopoc/runny/issues/45)) ([6012c9b](https://github.com/theopoc/runny/commit/6012c9b5bad8b19330803719384f3f7384ffb35a))

## [0.2.0](https://github.com/theopoc/runny/compare/v0.1.1...v0.2.0) (2026-08-14)


### Features

* **tui:** stream command output live ([39fc68b](https://github.com/theopoc/runny/commit/39fc68b4622a966348ba3e7b3abb76c707c358e4))


### Bug Fixes

* **tui:** focus tasks after command enter ([5ebd0f1](https://github.com/theopoc/runny/commit/5ebd0f1f3d38101371fa476f22eb0d910e5a91f2))
* **tui:** remove slash from filter input ([9097780](https://github.com/theopoc/runny/commit/90977803604650ff68d5b6ee2444211366eb2e57))
* **tui:** show full command palette ([4c09f20](https://github.com/theopoc/runny/commit/4c09f2001041f30d9f9073a7f919ccbbb874cb27))

## [0.1.1](https://github.com/theopoc/runny/compare/v0.1.0...v0.1.1) (2026-08-14)


### Bug Fixes

* harden execution safety ([e03cf09](https://github.com/theopoc/runny/commit/e03cf094cc70a961ea6b2fb22a69fffdfdb4f88e))
* **tui:** default quit confirmation to yes ([462ab8b](https://github.com/theopoc/runny/commit/462ab8b443066433bebba0c3c9a6156a2562afa5))
* **tui:** improve command editing and running feedback ([c4f149d](https://github.com/theopoc/runny/commit/c4f149d8348d0ffe40c4872036a6c60d16979189))
* **tui:** match terminal command editing behavior ([6579c18](https://github.com/theopoc/runny/commit/6579c18abd6a0a27387c837b83833d986b0c8f12))
* **tui:** preserve full command selection ([2e82ee7](https://github.com/theopoc/runny/commit/2e82ee74457e7e80330a0f1606026c71cb95ba35))

## 0.1.0 (2026-07-11)


### Features

* add docker usage ([b426622](https://github.com/theopoc/runny/commit/b4266223c3f246188ee084c38f32e3252f20d524))
* implement runny cli tui ([dd739fb](https://github.com/theopoc/runny/commit/dd739fb7425c4468331262cebbc6bf79531c1578))
* **runner:** honor log options ([64c7f7b](https://github.com/theopoc/runny/commit/64c7f7b3af9e3d3b13159884bb0ad09e55be3f6f))
* **tui:** add hierarchical task selection ([d22b322](https://github.com/theopoc/runny/commit/d22b32203ddf4d14341e0e0b75e04ec6207f7289))
* **tui:** align shortcut footer grid ([c893197](https://github.com/theopoc/runny/commit/c89319791da437c701040396245cd0abb1eb8489))
* **tui:** confirm quit with danger overlay ([6db927d](https://github.com/theopoc/runny/commit/6db927d03df9ce96074cb4b965848f9ba68a27eb))
* **tui:** emphasize command input ([86e86d5](https://github.com/theopoc/runny/commit/86e86d5f4c432ceba9acbf99aaeddd4c1ab6d905))
* **tui:** emphasize focused panel border ([d5aef63](https://github.com/theopoc/runny/commit/d5aef6377c9f9113f1887ad7edc0d9017affe064))
* **tui:** improve runny terminal experience ([ef8c719](https://github.com/theopoc/runny/commit/ef8c7191a61f10ff48069db110216fd81cdf1155))
* **tui:** persist command history ([f5908bb](https://github.com/theopoc/runny/commit/f5908bb48d79cfcd00569c745016766b0c8d8f36))
* **tui:** polish dashboard and directory styling ([fdc0681](https://github.com/theopoc/runny/commit/fdc06814b793a4bcf2763cf03455092bf39d9712))
* **tui:** polish dashboard layout ([ebe7bd6](https://github.com/theopoc/runny/commit/ebe7bd607836f3bf92fc13ed0967fc3c93191b9a))
* **tui:** redesign runny as tui-only ([80dc2bc](https://github.com/theopoc/runny/commit/80dc2bc53697d3ca07477ca46cbfeaa49f8ed25c))
* **tui:** refine dashboard directory styling ([f95ff67](https://github.com/theopoc/runny/commit/f95ff676ab336d0a898a309d2ea4d31c89ccdd13))
* **tui:** render help as overlay ([e327caa](https://github.com/theopoc/runny/commit/e327caa854b214ccba236218980390e7303c3b99))
* **tui:** run commands interactively ([d261c64](https://github.com/theopoc/runny/commit/d261c6459783889c941313dd3c67225ac1076366))
* **tui:** show output panel only ([0ff7b45](https://github.com/theopoc/runny/commit/0ff7b456aab4a9d15f2644f49ed7b1f91f659889))
* **tui:** show project run history ([70003f6](https://github.com/theopoc/runny/commit/70003f6b2a7e5cb716a90be66d07afb72927237d))
* **tui:** switch output tail shortcut to f ([6b24b24](https://github.com/theopoc/runny/commit/6b24b24cdbf7109a21278952a6aa86a1211a7e49))


### Bug Fixes

* **config:** default discovery depth to 3 ([5a74053](https://github.com/theopoc/runny/commit/5a74053f5c2d4fcccc94578aaf622fe6afd5ba9f))
* **runner:** cancel process groups ([4f8fd6d](https://github.com/theopoc/runny/commit/4f8fd6d573870b03a05312a5855329c7836e5d0f))
* **tui:** add directory navigation ([09252a2](https://github.com/theopoc/runny/commit/09252a22032fa84ad2aeafb6175abc5ff8171bab))
* **tui:** adjust running status colors ([a73c867](https://github.com/theopoc/runny/commit/a73c867355ac28be0d5a972eacd343568ce7a30a))
* **tui:** cancel selected running targets ([34b792e](https://github.com/theopoc/runny/commit/34b792e5913ec690061cb58b2192cf352beab6dc))
* **tui:** clarify cancel shortcut help ([b0fd3c3](https://github.com/theopoc/runny/commit/b0fd3c3e8ef7c7927ef83b4d2ae3a8ddaca83d11))
* **tui:** clarify zoom shortcut label ([14d6832](https://github.com/theopoc/runny/commit/14d6832dad7983edb93dc3491fbb219af1f207b5))
* **tui:** correct footer helper styling ([d8f661e](https://github.com/theopoc/runny/commit/d8f661e57c8b1422860ed7cf6e7581da144baf76))
* **tui:** force truecolor footer rendering ([55c5963](https://github.com/theopoc/runny/commit/55c596389f4287fc7dd7c8abc03bd2d7dd7959d7))
* **tui:** handle ctrl-c from overlays ([1fb278e](https://github.com/theopoc/runny/commit/1fb278e0243be60408e781e3bf3e49248b5f66d9))
* **tui:** hide success activity marker ([#3](https://github.com/theopoc/runny/issues/3)) ([ed0da5b](https://github.com/theopoc/runny/commit/ed0da5beeb669095a46f7a0b78b2885be858fd48))
* **tui:** keep directory cursor visible ([ae1d50a](https://github.com/theopoc/runny/commit/ae1d50a8fa4db18a98c58e77bf7f7072fba4334e))
* **tui:** keep parent context when filtering ([2f7aad6](https://github.com/theopoc/runny/commit/2f7aad63fe26b34721ff8a3f1313a12a902c7c73))
* **tui:** polish fullscreen dashboard ([ec421e8](https://github.com/theopoc/runny/commit/ec421e860e8e09a3c759094d60e1da1721b17f45))
* **tui:** remove selected marker background ([699c8f2](https://github.com/theopoc/runny/commit/699c8f2dd81dfbe4fed3debcebc84994bfc50695))
* **tui:** render fullscreen interface ([8e3b547](https://github.com/theopoc/runny/commit/8e3b547fe1edf3300cf3e07534007b6519204d00))
* **tui:** respect worker scheduling ([10b4171](https://github.com/theopoc/runny/commit/10b41710936d5f585a2c2b366f4d1aa0e2144361))
* **tui:** route text input by focus ([c609b46](https://github.com/theopoc/runny/commit/c609b4626e2d1870ae0a3641e8f27a1225f77a50))
* **tui:** scope progress to selected targets ([c9ee066](https://github.com/theopoc/runny/commit/c9ee066f9d338b1bbaddeed66076715d1e82eb07))
* **tui:** show execution settings ([328ae91](https://github.com/theopoc/runny/commit/328ae9128ecfeeccf5af8015ed81c256c579cd44))
* **tui:** show only command in command bar ([d30f1a6](https://github.com/theopoc/runny/commit/d30f1a63f1cc6ed2c11b44eb9369b311d818dc90))
* **tui:** show trailing command spaces ([a35c275](https://github.com/theopoc/runny/commit/a35c2753566b451e658ba80ed2216266624cc64a))
* **tui:** simplify command preview ([02874f8](https://github.com/theopoc/runny/commit/02874f838088a28b5d5778376095f78e8b81f044))
* **tui:** simplify dashboard headers ([c0a4ff2](https://github.com/theopoc/runny/commit/c0a4ff2f54cd71698c576aeb9305cd929b735c62))
* **tui:** start with tasks focused ([f3ce06c](https://github.com/theopoc/runny/commit/f3ce06ce6312c9f3c4183c40876ff83a520a6749))
* **tui:** use blue focus highlight ([849f074](https://github.com/theopoc/runny/commit/849f074e682b7c2ccfbed5c860aa62e94ac74281))
