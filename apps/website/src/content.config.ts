import { defineCollection, z } from "astro:content";
import { glob } from "astro/loaders";

const docs = defineCollection({
  loader: glob({ pattern: "**/*.{md,mdx}", base: "./src/content/docs" }),
  schema: z.object({
    title: z.string(),
    description: z.string(),
    group: z.enum(["Start", "Guides", "Reference"]),
    order: z.number(),
    readTime: z.string().optional(),
    note: z.string().optional(),
    progress: z.array(z.string()).optional(),
  }),
});

export const collections = { docs };
