import { useState } from "react";
import axios from "axios";



function Signup() {
  const [email, setEmail] = useState("");
  const [name, setName] = useState("");
  const [password, setPassword] = useState("");
  const [file, setFile] = useState<File | null>()

  const handleSignup = async (e: React.FormEvent) => {
    e.preventDefault();
    if(!file){
      alert("Select a file !")
      return
    }

    const formData = new FormData();
    
    formData.append("pfp", file); 
    formData.append("email", email)
    formData.append("name", name)
    formData.append("password", password)

    try {
      const response = await axios.post("http://localhost:3000/test/file", formData, {
        headers: {
          "Content-Type": "multipart/form-data",
        }
      });
      console.log("Upload success:", await response.data);
    } catch(e){
      console.log(e);
    }
  };

  return (
    <div className="min-h-screen bg-lime-100 flex items-center justify-center">
      <div className="w-[90%] max-w-md bg-white shadow-lg rounded-lg border border-gray-300 p-6">
        <form
          onSubmit={handleSignup}
          className="space-y-4"
          encType="multipart/form-data"
        >
          <h2 className="text-2xl font-bold text-center text-blue-600">
            Sign Up
          </h2>

          <input
            type="email"
            name="email"
            id="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            placeholder="Email"
            className="w-full px-4 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
          />

          <input
            type="text"
            name="name"
            id="name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="Full Name"
            className="w-full px-4 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
          />

          <input
            type="password"
            name="password"
            id="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder="Password"
            className="w-full px-4 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
          />

          <input
            type="file"
            accept="image/*"
            name="pfp"
            id="pfp"
            onChange={(e) => setFile(e.target.files?.[0] || null)}
            className="w-full px-4 py-2 border rounded-md bg-gray-50 cursor-pointer file:mr-4 file:py-2 file:px-4 file:border-0 file:text-sm file:font-semibold file:bg-blue-50 file:text-blue-700 hover:file:bg-blue-100"
          />

          <button
            type="submit"
            className="w-full py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 transition"
          >
            Submit
          </button>
        </form>
      </div>
    </div>
  );
}

export default Signup;
