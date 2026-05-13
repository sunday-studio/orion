import { useState, Fragment } from "react";
import { useGetAgents } from "../../orion-sdk";

export function HomePage() {
  const { data, isLoading, error } = useGetAgents();


  console.log(data);
  return (
    <div>
      <h1>Home</h1>
    </div>
  );
}
