/** "1 monitor", "0 monitors", "2 monitors". */
export const pluralize = (count: number, singular: string, plural = `${singular}s`) =>
  `${count} ${count === 1 ? singular : plural}`;
