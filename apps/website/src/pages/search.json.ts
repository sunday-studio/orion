import { getCollection, render } from "astro:content";

export async function GET() {
  const docs = (await getCollection("docs")).sort((a, b) => a.data.order - b.data.order);
  const index = await Promise.all(
    docs.map(async (entry) => {
      const { headings } = await render(entry);
      return {
        title: entry.data.title,
        description: entry.data.description,
        group: entry.data.group,
        url: `/docs/${entry.id}/`,
        headings: headings.filter((h) => h.depth === 2).map((h) => ({ text: h.text, slug: h.slug })),
      };
    }),
  );
  return new Response(JSON.stringify(index), { headers: { "Content-Type": "application/json" } });
}
