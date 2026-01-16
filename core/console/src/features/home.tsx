import { useGetV1Agents } from '../api/generated/agents/agents';

export const Home = () => {
  const { data, isLoading, error } = useGetV1Agents(
    { limit: 10, offset: 0 },
    {
      query: {
        refetchOnWindowFocus: false,
      },
    }
  );

  if (isLoading) {
    return <div>Loading agents...</div>;
  }

  if (error) {
    return <div>Error loading agents</div>;
  }

  // The response structure is: data.data.data.agents
  const responseData = data?.data?.data;
  const agents = (responseData && 'agents' in responseData ? responseData.agents : []) || [];
  const count = (responseData && 'count' in responseData ? responseData.count : 0) || 0;

  return (
    <div>
      <h1>Orion Core Dashboard</h1>
      <h2>Agents ({count})</h2>
      <ul>
        {agents.map((agent) => (
          <li key={agent.id}>
            {agent.name} - {agent.os} ({agent.arch})
          </li>
        ))}
      </ul>
    </div>
  );
};
