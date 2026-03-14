import Nav from "./components/nav";
import Hero from "./components/hero";
import Install from "./components/install";
import Pools from "./components/pools";
import Commands from "./components/commands";
import Concepts from "./components/concepts";
import Footer from "./components/footer";

export default function Home() {
  return (
    <>
      <Nav />
      <Hero />
      <Install />
      <Pools />
      <Commands />
      <Concepts />
      <Footer />
    </>
  );
}
