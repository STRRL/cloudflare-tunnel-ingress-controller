import type { StarlightPlugin } from "@astrojs/starlight/types";

export default function starlightOGImages(): StarlightPlugin {
  return {
    name: "starlight-og-images",
    hooks: {
      setup({ config, updateConfig }) {
        updateConfig({
          head: [
            ...config.head,
            {
              tag: "meta",
              attrs: {
                property: "og:image",
                content: "/og/$slug.png",
              },
            },
            {
              tag: "meta",
              attrs: {
                property: "twitter:image",
                content: "/og/$slug.png",
              },
            },
          ],
        });
      },
    },
  };
}
