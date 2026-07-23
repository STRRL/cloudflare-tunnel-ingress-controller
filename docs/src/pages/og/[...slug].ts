import { getCollection } from "astro:content";
import { OGImageRoute } from "astro-og-canvas";

const entries = await getCollection("docs");

const pages = Object.fromEntries(entries.map(({ data, id }) => [id, { data }]));

export const { getStaticPaths, GET } = OGImageRoute({
  pages,
  param: "slug",
  getImageOptions: (_path, page: (typeof pages)[string]) => {
    return {
      title: page.data.title,
      description: page.data.description,
      bgGradient: [[24, 24, 27]],
      border: { color: [63, 63, 70], width: 20 },
      padding: 120,
    };
  },
});
